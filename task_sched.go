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
	schedLogText     = widget.NewMultiLineEntry() // 用于显示日志信息
	scanButton       *widget.Button               // 扫描按钮
	progressBar      *widget.ProgressBarInfinite  // 无限进度条
	stopChan         chan struct{}                // 用于停止任务的通道
	wg               sync.WaitGroup               // 用于等待任务完成的 WaitGroup
	orderNumberEntry *widget.Entry                // 用于输入编号的输入框
)

// 创建界面 UI
func createSchedUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	// 使用配置中的日志路径初始化日志文件
	InitSchedLogger(utils.SchedLogPath)

	// 更新任务结束时的UI状态
	updateUIOnTaskEnd := func() {
		scanButton.SetText("开始任务")
		scanButton.Importance = widget.MediumImportance
		scanButton.Refresh()
		SchedLogToFile("任务已停止")
		progressBar.Stop()
	}

	// 启动自动任务的功能
	startSchedTask := func() {
		stopChan = make(chan struct{})

		// 显示无限进度条
		scanButton.SetText("停止任务")
		scanButton.Importance = widget.HighImportance
		scanButton.Refresh()
		progressBar.Start() // 启动进度条

		wg.Add(1)
		go func() {
			defer wg.Done()

			// 加载配置，获取最大执行次数
			newConfig, err := LoadConfig("config.json")
			if err != nil {
				SchedLogToFile(fmt.Sprintf("加载配置失败: %s", err.Error()))
				updateUIOnTaskEnd()
				return
			}
			maxExecutions, err := strconv.Atoi(newConfig.SchedTimes)
			if err != nil || maxExecutions <= 0 {
				SchedLogToFile(fmt.Sprintf("无效的执行次数: %s", newConfig.SchedTimes))
				updateUIOnTaskEnd()
				return
			}

			for executionCount := 0; executionCount < maxExecutions; executionCount++ {
				select {
				case <-stopChan:
					return
				default:
					// 每轮重新加载配置，允许任务期间动态更新配置
					newConfig, err = LoadConfig("config.json")
					if err != nil {
						SchedLogToFile(fmt.Sprintf("加载配置失败: %s", err.Error()))
						updateUIOnTaskEnd()
						return
					}

					if newConfig.IOBuffer <= 0 {
						SchedLogToFile("缓冲区大小必须大于零")
						updateUIOnTaskEnd()
						return
					}

					SchedLogToFile(fmt.Sprintf("第 %d 次任务开始...", executionCount+1))

					// 获取输入框中的编号
					orderNumbers := orderNumberEntry.Text

					SchedLogToFile("开始扫描和复制文件...")
					// 打印需要匹配的编号
					if orderNumbers != "" {
						SchedLogToFile(fmt.Sprintf("需要匹配的编号: %s", orderNumbers))
					} else {
						SchedLogToFile("未输入编号，不进行编号匹配")
					}
					// 传递编号给 ScanAndCopyFolders 函数
					err = ScanAndCopyFolders(newConfig, orderNumbers)
					if err != nil {
						SchedLogToFile(fmt.Sprintf("扫描和复制文件失败: %s", err.Error()))
						updateUIOnTaskEnd()
						return
					}
					SchedLogToFile("文件扫描和复制完成")
					time.Sleep(500 * time.Millisecond)

					SchedLogToFile("开始处理图像...")
					err = HandleImages(newConfig.LocalFolder, newConfig.PicCompress, newConfig.PicWidth, newConfig.PicSize, true)
					if err != nil {
						SchedLogToFile(fmt.Sprintf("处理图像失败: %s", err.Error()))
					} else {
						SchedLogToFile("图像处理完成")
					}
					time.Sleep(500 * time.Millisecond)

					SchedLogToFile("开始上传图片到OSS...")
					err = UploadImagesWithTaskType(newConfig, true)
					if err != nil {
						if err.Error() == "无文件可上传" {
							SchedLogToFile("无文件可上传")
						} else {
							SchedLogToFile(fmt.Sprintf("上传图片失败: %v，\n20 秒后重试一次...", err))
							time.Sleep(20 * time.Second)
							err = UploadImagesWithTaskType(newConfig, true)
							if err != nil {
								SchedLogToFile(fmt.Sprintf("重试仍然失败: %v", err))
								if notifyErr := newConfig.NotifyUploadFailed(); notifyErr != nil {
									SchedLogToFile(fmt.Sprintf("发送企业微信通知: %v", notifyErr))
								}
							} else {
								SchedLogToFile("重试成功，所有图片上传完成")
							}
						}
					}

					currentTime := time.Now().Format("2006.01.02 15:04:05")
					SchedLogToFile(fmt.Sprintf("%s 当前执行周期已完成 (%d/%d)", currentTime, executionCount+1, maxExecutions))

					// 不等待间隔，立即进入下一轮或结束
				}
			}

			SchedLogToFile("所有计划任务已完成 ✅")
			updateUIOnTaskEnd()
		}()

	}

	// 创建日期 UI 组件，并获取 startTimeLabel
	dateUI := CreateDateUI(myWindow, config)

	// 设置日志文本框
	schedLogText.SetMinRowsVisible(15)

	// 初始化进度条并
	progressBar = widget.NewProgressBarInfinite()
	progressBar.Stop() // 确保进度条初始为停止状态

	// 初始化停止通道
	stopChan = make(chan struct{})

	// 任务按钮的启停逻辑部分
	scanButton = widget.NewButton("开始任务", func() {
		if scanButton.Text == "开始任务" {
			dialog.ShowConfirm("确认开始", "确定要开始任务吗？", func(confirm bool) {
				if confirm {
					startSchedTask() // 使用局部函数
				}
			}, myWindow)
		} else {
			// 停止任务
			dialog.ShowConfirm("确认停止", "确定要停止任务吗？", func(confirm bool) {
				if confirm {
					SchedLogToFile("正在停止任务...")

					// 关闭停止通道
					close(stopChan)

					// 等待任务完成
					wg.Wait()

					// 直接更新UI状态
					updateUIOnTaskEnd()
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

	// 创建 schedTimes 文本框和保存按钮
	schedTimesEntry := widget.NewEntry()
	schedTimesEntry.SetText(config.SchedTimes)

	// 创建标签
	TimesLabel := widget.NewLabel("执行次数:")
	sLabel := widget.NewLabel("次")

	// 设置输入框的宽度
	schedTimesContainer := container.NewHBox(
		TimesLabel,
		container.NewGridWrap(fyne.NewSize(120, utils.LEBHeight), schedTimesEntry),
		sLabel,
	)

	saveButton := widget.NewButton("修改执行次数", func() {
		dialog.ShowConfirm("确认保存", "确定要保存配置吗？", func(confirm bool) {
			if confirm {
				// 验证输入
				interval, err := strconv.Atoi(schedTimesEntry.Text)
				if err != nil || interval <= 0 {
					dialog.ShowInformation("输入错误", "请输入有效的循环执行次数（正整数）", myWindow)
					return
				}

				// 直接更新当前配置
				config.SchedTimes = schedTimesEntry.Text

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
	TimesContainer := container.NewVBox(
		schedTimesContainer,
		saveButton,
	)

	// 编号输入框
	orderNumberEntry = widget.NewEntry()
	orderNumberEntry.SetPlaceHolder("编号匹配：输入一个或多个编号(逗号分割)，可模糊匹配。处理日期范围内所有文件请留空，勿输入空格")

	// 初始化无限进度条
	progressBar.Start()
	time.Sleep(10 * time.Millisecond) // 只需要很短的时间
	progressBar.Stop()

	// 创建 "任务界面" Tab 内容，将日期 UI 放在文件夹扫描器 UI 之前
	return container.NewVBox(
		container.NewBorder(nil, nil, nil, TimesContainer, dateUI), // 将保存配置的 UI 组件布局在日期 UI 的右边
		orderNumberEntry, // 添加编号输入框容器
		folderScannerUI,
		schedLogText,
	)
}
