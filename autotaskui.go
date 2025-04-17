package main

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"go-uposs/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var (
	autoLogText     = widget.NewMultiLineEntry() // 用于显示日志信息
	autoScanButton  *widget.Button               // 扫描按钮
	autoProgressBar *widget.ProgressBarInfinite  // 无限进度条
	autoStopChan    chan struct{}                // 用于停止任务的通道
	autoWg          sync.WaitGroup               // 用于等待任务完成的 WaitGroup
)

// 创建界面 UI
func createautoTaskUI(myWindow fyne.Window, config *Config) fyne.CanvasObject {
	// 使用配置中的日志路径初始化日志文件
	InitAutoLogger(utils.AutoLogPath)

	// 更新任务结束时的UI状态
	updateUIOnTaskEnd := func() {
		autoProgressBar.Stop()
		autoScanButton.SetText("开始任务")
		autoScanButton.Importance = widget.MediumImportance
		autoScanButton.Refresh()

		AutoLogToFile("任务已停止")
	}

	// 启动自动任务的功能
	startAutoTask := func() {
		autoStopChan = make(chan struct{})

		// 更新UI状态
		autoProgressBar.Start()
		autoScanButton.SetText("停止任务")
		autoScanButton.Importance = widget.HighImportance
		autoScanButton.Refresh()

		// 记录日志
		AutoLogToFile("开始自动任务")

		autoWg.Add(1)
		go func() {
			defer autoWg.Done()
			for {
				select {
				case <-autoStopChan:
					return
				default:
					// 加载最新配置 - 直接使用文件名
					newConfig, err := LoadConfig("config.json")
					if err != nil {
						AutoLogToFile(fmt.Sprintf("加载配置失败: %s", err.Error()))
						// 直接在这里更新UI状态
						updateUIOnTaskEnd()
						return
					}

					// 检查缓冲区大小
					if newConfig.IOBuffer <= 0 {
						AutoLogToFile("缓冲区大小必须大于零")
						updateUIOnTaskEnd()
						return
					}

					// 步骤1: 扫描和复制文件
					AutoLogToFile("开始扫描和复制文件...")

					err = ScanAndCopyFoldersForToday(newConfig)
					if err != nil {
						AutoLogToFile(fmt.Sprintf("扫描和复制文件失败: %s", err.Error()))
						updateUIOnTaskEnd()
						return
					}
					AutoLogToFile("文件扫描和复制完成")

					// 步骤2: 处理图像
					AutoLogToFile("开始处理图像...")

					err = HandleImages(newConfig.LocalFolder, newConfig.PicCompress, newConfig.PicWidth, newConfig.PicSize, false)
					if err != nil {
						AutoLogToFile(fmt.Sprintf("处理图像失败: %s", err.Error()))
						updateUIOnTaskEnd()
						return
					}
					AutoLogToFile("图像处理完成")

					// 步骤3: 上传图片
					AutoLogToFile("开始上传图片到OSS...")

					err = UploadImagesWithTaskType(newConfig, false) // 指定为自动任务
					if err != nil {
						if err.Error() == "无文件可上传" {
							AutoLogToFile("无文件可上传")
						} else {
							AutoLogToFile(fmt.Sprintf("上传图片失败: %v，\n20 秒后重试一次...", err))
							time.Sleep(20 * time.Second) // 等待 20 秒再试一次

							// 再次尝试
							err = UploadImagesWithTaskType(newConfig, false)
							if err != nil {
								AutoLogToFile(fmt.Sprintf("重试仍然失败: %v", err))
								// 发送企业微信通知
								if notifyErr := newConfig.NotifyUploadFailed(); notifyErr != nil {
									AutoLogToFile(fmt.Sprintf("发送企业微信通知: %v", notifyErr))
								}
							} else {
								AutoLogToFile("重试成功，所有图片上传完成")
							}

						}
					} else {
						AutoLogToFile("所有图片上传完成")
					}

					// 当前执行周期完成
					AutoLogToFile("当前执行周期已完成")

					// 获取间隔时间
					interval, err := strconv.Atoi(newConfig.AutoInterval)
					if err != nil {
						AutoLogToFile(fmt.Sprintf("无效的间隔时间: %s", newConfig.AutoInterval))
						updateUIOnTaskEnd()
						return
					}

					AutoLogToFile(fmt.Sprintf("将在 %d 秒后开始下一次任务执行...", interval))

					select {
					case <-autoStopChan:
						return
					case <-time.After(time.Duration(interval) * time.Second):
						// 继续下一个循环
					}
				}
			}
		}()
	}

	timeBind := binding.NewString()                // 创建一个新的字符串绑定
	timeLabel := widget.NewLabelWithData(timeBind) // 创建一个新的标签，并绑定到时间数据

	go func() { // 启动一个 goroutine 来更新时间
		for t := range time.Tick(time.Second) { // 每秒更新一次标签的值
			_ = timeBind.Set(t.Format("2006-01-02 15:04:05"))
		}
	}()

	// 系统时间标签
	systemTimeLabel := widget.NewLabel("系统时间:")

	// 设置日志文本框
	autoLogText.SetMinRowsVisible(20)

	// 初始化进度条
	autoProgressBar = widget.NewProgressBarInfinite()
	autoProgressBar.Stop() // 确保进度条初始为停止状态

	// 初始化停止通道
	autoStopChan = make(chan struct{})

	// 任务按钮的启停逻辑部分
	autoScanButton = widget.NewButton("开始任务", func() {
		if autoScanButton.Text == "开始任务" {
			dialog.ShowConfirm("确认开始", "确定要开始任务吗？", func(confirm bool) {
				if confirm {
					startAutoTask() // 使用局部函数
				}
			}, myWindow)
		} else {
			// 停止任务
			dialog.ShowConfirm("确认停止", "确定要停止任务吗？", func(confirm bool) {
				if confirm {
					AutoLogToFile("正在停止任务...")

					// 关闭停止通道
					close(autoStopChan)

					// 等待任务完成
					autoWg.Wait()

					// 直接更新UI状态
					updateUIOnTaskEnd()
				}
			}, myWindow)
		}
	})

	// 为需要手动触发自动任务的地方提供访问点，例如，当程序启动时自动执行任务
	// 使用闭包方式，避免全局函数
	triggerAutoTask = startAutoTask

	// 设置按钮和进度条的宽度
	scanButtonContainer := container.NewGridWrap(fyne.NewSize(300, 35), autoScanButton)
	progressBarContainer := container.NewGridWrap(fyne.NewSize(485, 35), autoProgressBar)

	// 创建文件夹扫描器 UI 组件布局
	folderScannerUI := container.NewHBox(
		scanButtonContainer,  // 扫描按钮
		progressBarContainer, // 无限进度条
	)

	// 初始化无限进度条并确保其处于停止状态
	autoProgressBar.Start()
	time.Sleep(10 * time.Millisecond) // 只需要很短的时间
	autoProgressBar.Stop()

	// 创建 sched_interval 文本框和保存按钮
	schedIntervalEntry := widget.NewEntry()
	schedIntervalEntry.SetText(config.AutoInterval)

	// 创建标签
	intervalLabel := widget.NewLabel("执行间隔:")
	sLabel := widget.NewLabel("s")

	// 设置输入框的宽度
	schedIntervalContainer := container.NewHBox(
		intervalLabel,
		container.NewGridWrap(fyne.NewSize(150, utils.LEBHeight), schedIntervalEntry),
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

				// 更新配置
				config.AutoInterval = schedIntervalEntry.Text

				// 保存配置 - 直接使用文件名
				if err := SaveConfig("config.json", config); err != nil {
					dialog.ShowInformation("保存失败", fmt.Sprintf("保存配置失败: %v", err), myWindow)
				} else {
					AutoLogToFile("配置已成功保存")
				}
			}
		}, myWindow)
	})

	// 设置按钮的宽度
	saveButtonContainer := container.NewGridWrap(fyne.NewSize(150, utils.LEBHeight), saveButton)

	// 创建按钮容器，按钮上下排列，并设置按钮的尺寸
	intervalContainer := container.NewBorder(nil, nil, nil, nil, container.NewHBox(
		schedIntervalContainer,
		saveButtonContainer,
	))

	// 创建 "任务界面" Tab 内容，将日期 UI 放在文件夹扫描器 UI 之前
	ui := container.NewVBox(
		container.NewBorder(nil, nil, nil, intervalContainer, container.NewHBox(systemTimeLabel, timeLabel)), // 系统时间标签、实时时间标签和输入框、保存按钮
		folderScannerUI,
		autoLogText,
	)

	return ui
}

// triggerAutoTask 是一个函数变量，用于从外部触发自动任务
// 例如，当程序启动时自动执行任务
var triggerAutoTask func()
