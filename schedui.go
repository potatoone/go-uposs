package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"go-uposs/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var (
	schedLogText = widget.NewMultiLineEntry() // 用于显示日志信息
	progressBar  *widget.ProgressBarInfinite  // 无限进度条
	stopChan     chan struct{}                // 用于停止任务的通道
	wg           sync.WaitGroup               // 用于等待任务完成的 WaitGroup
	scanButton   *widget.Button               // 扫描按钮
)

// updateUIOnTaskEnd 更新任务结束时的UI状态
func updateUIOnTaskEnd() {
	progressBar.Stop()
	scanButton.SetText("开始任务")
	scanButton.Importance = widget.MediumImportance
	scanButton.Refresh()
	SchedLogToFile("任务已停止")
}

// 创建界面 UI
func createSchedUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	// 使用配置中的日志路径初始化日志文件
	InitSchedLogger(utils.SchedLogPath)

	// 创建日期 UI 组件，并获取 startTimeLabel
	dateUI := CreateDateUI(myWindow, config)

	// 设置日志文本框
	schedLogText.SetMinRowsVisible(17)

	// 初始化进度条并
	progressBar = widget.NewProgressBarInfinite()
	progressBar.Stop() // 确保进度条初始为停止状态

	// 初始化停止通道
	stopChan = make(chan struct{})

	// 扫描按钮
	scanButton = widget.NewButton("开始任务", func() {
		if scanButton.Text == "开始任务" {
			dialog.ShowConfirm("确认开始", "确定要开始任务吗？", func(confirm bool) {
				if confirm {
					// 显示无限进度条
					progressBar.Start() // 启动进度条
					scanButton.SetText("停止任务")
					scanButton.Importance = widget.HighImportance
					scanButton.Refresh()

					wg.Add(1)
					go func() {
						defer wg.Done()
						for {
							select {
							case <-stopChan:
								return
							default:
								// 加载最新配置
								newConfig, err := LoadConfig("config.json") // 只传递文件名
								if err != nil {
									SchedLogToFile(fmt.Sprintf("加载配置失败: %s", err.Error()))
									updateUIOnTaskEnd()
									return
								}

								// 检查配置有效性
								if newConfig.IOBuffer <= 0 {
									SchedLogToFile("缓冲区大小必须大于零")
									updateUIOnTaskEnd()
									return
								}

								// 步骤1: 首先扫描和复制文件
								SchedLogToFile("开始扫描和复制文件...")
								err = ScanAndCopyFolders(newConfig)
								if err != nil {
									SchedLogToFile(fmt.Sprintf("扫描和复制文件失败: %s", err.Error()))
									updateUIOnTaskEnd()
									return
								}
								SchedLogToFile("文件扫描和复制完成")

								// 添加短暂延迟确保文件操作完成
								time.Sleep(500 * time.Millisecond)

								// 步骤2: 然后处理图像
								SchedLogToFile("开始处理图像...")
								err = HandleImages(newConfig.LocalFolder, newConfig.PicCompress, newConfig.PicWidth, newConfig.PicSize, true)
								if err != nil {
									SchedLogToFile(fmt.Sprintf("处理图像失败: %s", err.Error()))
								} else {
									SchedLogToFile("图像处理完成")
								}

								// 添加短暂延迟确保文件操作完成
								time.Sleep(500 * time.Millisecond)

								// 步骤3: 最后上传图片
								SchedLogToFile("开始上传图片到OSS...")
								err = UploadImagesWithTaskType(newConfig, true) // 指定为计划任务
								if err != nil {
									if err.Error() == "无文件可上传" {
										// 特殊处理"无文件可上传"的情况
										SchedLogToFile("无文件可上传")
									} else {
										// 其他错误，先记录错误，再尝试重试一次
										SchedLogToFile(fmt.Sprintf("上传图片失败: %v，\n20 秒后重试一次...", err))
										time.Sleep(20 * time.Second)

										// 再次尝试上传
										err = UploadImagesWithTaskType(newConfig, true)
										if err != nil {
											SchedLogToFile(fmt.Sprintf("重试仍然失败: %v", err))
											// 发送企业微信通知
											if notifyErr := newConfig.NotifyUploadFailed(); notifyErr != nil {
												SchedLogToFile(fmt.Sprintf("发送企业微信通知: %v", notifyErr))
											}
										} else {
											SchedLogToFile("重试成功，所有图片上传完成")
										}

									}
								} else {
									SchedLogToFile("所有图片上传完成")
								}

								// 当前执行周期完成
								currentTime := time.Now().Format("2006.01.02 15:04:05")
								SchedLogToFile(fmt.Sprintf("%s 当前执行周期已完成", currentTime))

								// 等待设定的间隔时间后执行下一次任务
								interval, err := strconv.Atoi(newConfig.SchedInterval)
								if err != nil {
									SchedLogToFile(fmt.Sprintf("无效的间隔时间: %s", newConfig.SchedInterval))
									updateUIOnTaskEnd()
									return
								}

								SchedLogToFile(fmt.Sprintf("将在 %d 秒后开始下一次任务执行...", interval))

								select {
								case <-stopChan:
									return
								case <-time.After(time.Duration(interval) * time.Second):
									// 继续下一个循环
								}
							}
						}
					}()
				}
			}, myWindow)
		} else {
			// 停止任务
			dialog.ShowConfirm("确认停止", "确定要停止任务吗？", func(confirm bool) {
				if confirm {
					SchedLogToFile("正在停止任务...")
					close(stopChan)
					wg.Wait()
					updateUIOnTaskEnd()
					stopChan = make(chan struct{}) // 重置通道
				}
			}, myWindow)
		}
	})

	// 设置按钮和进度条的宽度
	scanButtonContainer := container.NewGridWrap(fyne.NewSize(300, 35), scanButton) // 增加按钮宽度
	progressBarContainer := container.NewGridWrap(fyne.NewSize(485, 35), progressBar)

	// 创建文件夹扫描器 UI 组件布局 - 删除上传按钮
	folderScannerUI := container.NewHBox(
		scanButtonContainer,  // 扫描按钮
		progressBarContainer, // 无限进度条
	)

	// 初始化无限进度条
	progressBar.Start()
	time.Sleep(1 * time.Second)
	progressBar.Stop()

	// 创建 sched_interval 文本框和保存按钮
	schedIntervalEntry := widget.NewEntry()
	schedIntervalEntry.SetText(config.SchedInterval)

	// 创建标签
	intervalLabel := widget.NewLabel("执行间隔:")
	sLabel := widget.NewLabel("s")

	// 设置输入框的宽度
	schedIntervalContainer := container.NewHBox(
		intervalLabel,
		container.NewGridWrap(fyne.NewSize(120, utils.LEBHeight), schedIntervalEntry),
		sLabel,
	)

	saveButton := widget.NewButton("修改执行间隔", func() {
		dialog.ShowConfirm("确认保存", "确定要保存配置吗？", func(confirm bool) {
			if confirm {
				// 验证输入
				interval, err := strconv.Atoi(schedIntervalEntry.Text)
				if err != nil || interval <= 0 {
					dialog.ShowInformation("输入错误", "请输入有效的时间间隔（正整数）", myWindow)
					return
				}

				// 直接更新当前配置
				config.SchedInterval = schedIntervalEntry.Text

				// 保存配置 - 直接使用文件名
				if err := SaveConfig("config.json", config); err != nil {
					dialog.ShowInformation("保存失败", fmt.Sprintf("保存配置失败: %v", err), myWindow)
				} else {
					currentTime := time.Now().Format("2006.01.02 15:04:05")
					SchedLogToFile(fmt.Sprintf("%s 配置已成功保存", currentTime))
				}
			}
		}, myWindow)
	})

	// 创建按钮容器，按钮上下排列，并设置按钮的尺寸
	intervalContainer := container.NewVBox(
		schedIntervalContainer,
		saveButton,
	)

	// 创建 "任务界面" Tab 内容，将日期 UI 放在文件夹扫描器 UI 之前
	return container.NewVBox(
		container.NewBorder(nil, nil, nil, intervalContainer, dateUI), // 将保存配置的 UI 组件布局在日期 UI 的右边
		folderScannerUI,
		schedLogText,
	)
}
