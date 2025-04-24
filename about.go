package main

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"go-uposs/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// createCleanSettingsUI 创建数据清理设置UI
func createCleanSettingsUI(win fyne.Window) fyne.CanvasObject {
	config, err := LoadConfig("config.json")
	if err != nil {
		dialog.ShowError(fmt.Errorf("加载配置失败: %v", err), win)
		return nil
	}

	// 添加编号输入文本框
	orderNumberEntry := widget.NewEntry()
	orderNumberEntry.SetPlaceHolder("数据清理：不输入默认清理选择日期范围的所有记录，可输入一个或多个编号（逗号分割）")

	// 直接将日期选择UI放在界面上，使用专门的清理日期UI
	dateUI := CreateCleanDateUI(win, config)

	aboutLogText := widget.NewMultiLineEntry()
	aboutLogText.SetMinRowsVisible(6)

	// 添加清除日志文件复选框
	clearLogCheck := widget.NewCheck("同时删除日志", nil)
	clearLogCheck.SetChecked(false) // 默认不清除日志文件

	cleanBtn := widget.NewButton("执行清理", func() {
		// 获取最新的清理日期范围
		startTimeText := config.CleanStartTime
		endTimeText := config.CleanEndTime

		// 解析日期
		startDate, err := time.Parse("2006.01.02", startTimeText)
		if err != nil {
			dialog.ShowError(fmt.Errorf("开始日期格式错误: %v", err), win)
			return
		}

		endDate, err := time.Parse("2006.01.02", endTimeText)
		if err != nil {
			dialog.ShowError(fmt.Errorf("结束日期格式错误: %v", err), win)
			return
		}

		// 验证日期范围
		if endDate.Before(startDate) {
			dialog.ShowError(fmt.Errorf("结束日期不能早于开始日期"), win)
			return
		}

		// 确保结束日期不超过当前日期
		if endDate.After(time.Now()) {
			dialog.ShowError(fmt.Errorf("结束日期不能晚于今天"), win)
			return
		}

		// 获取编号输入并处理
		orderNumbersInput := strings.TrimSpace(orderNumberEntry.Text)
		var orderNumbers []string
		if orderNumbersInput != "" {
			orderNumbers = strings.Split(orderNumbersInput, ",")
			for i := range orderNumbers {
				orderNumbers[i] = strings.TrimSpace(orderNumbers[i])
			}
		}

		// 根据复选框状态构建确认消息
		var confirmMsg string
		if clearLogCheck.Checked {
			confirmMsg = fmt.Sprintf("将清理从 %s 到 %s 的所有日志文件和数据库记录，是否继续？",
				startTimeText, endTimeText)
		} else {
			confirmMsg = fmt.Sprintf("将清理从 %s 到 %s 的数据库记录（不包含日志文件），是否继续？",
				startTimeText, endTimeText)
		}

		dialog.ShowConfirm("确认清理", confirmMsg, func(confirmed bool) {
			if !confirmed {
				return
			}

			go func() {
				cleanCfg := CleanConfig{
					StartTime: startTimeText,
					EndTime:   endTimeText,
				}

				// 根据复选框决定是否清除日志文件
				var filesCount int
				var filesSize int64
				var logMsg string

				if clearLogCheck.Checked {
					filesCount, filesSize, _ = cleanLogFilesByDateRange(cleanCfg, false)
					logMsg = fmt.Sprintf("已清理从 %s 到 %s 的 %d 个日志文件 (%.2f MB)",
						startTimeText, endTimeText, filesCount, float64(filesSize)/(1024*1024))
					updateLog(aboutLogText, "[关于]", logMsg)
				}

				// 清理数据库记录，传递编号参数
				recordCount, err := cleanDbRecordsByDateAndNumbers(cleanCfg, orderNumbers, false)
				if err != nil {
					log.Printf("清理数据库记录出错: %v", err)
				}
				logMsg = fmt.Sprintf("已清理从 %s 到 %s 的 %d 条数据库记录",
					startTimeText, endTimeText, recordCount)
				if len(orderNumbers) > 0 {
					logMsg += fmt.Sprintf("，匹配编号: %s", strings.Join(orderNumbers, ", "))
				}
				updateLog(aboutLogText, "[关于]", logMsg)
			}()
		}, win)
	})

	// 将复选框放入一个容器
	checksContainer := container.NewVBox(clearLogCheck)

	// 使用GridWrap设置按钮大小，并放置在垂直容器中
	buttonContainer := container.NewVBox(
		container.NewGridWrap(fyne.NewSize(150, 40), cleanBtn),
		container.NewPadded(checksContainer), // 加入复选框
		layout.NewSpacer(),                   // 添加一个空间填充，确保按钮位于顶部
	)

	// 使用BorderLayout布局，将dateUI放在中间，将按钮放在右侧
	topContainer := container.NewBorder(
		nil, nil, nil, buttonContainer, // 上，下，左，右
		dateUI, // 中间部分
	)

	// 将日志输出控件放在主容器的底部
	mainContainer := container.NewVBox(
		orderNumberEntry,
		topContainer,
		container.NewVBox(
			aboutLogText,
		),
	)

	return mainContainer
}

// 修复时间戳重复问题

// 添加日志输出函数
func aboutLogToUIAndSystem(message string) {
	// 不在这里添加时间戳，只添加标识符
	formattedMessage := fmt.Sprintf("[关于] %s", message)

	// 写入系统日志，SysLogToFile 会添加时间戳
	SysLogToFile(formattedMessage)
}

// 创建关于界面的 UI
func createAboutUI(win fyne.Window) fyne.CanvasObject {
	// 加载配置
	config, err := LoadConfig("config.json")
	if err != nil {
		dialog.ShowError(fmt.Errorf("加载配置失败: %v", err), win)
		aboutLogToUIAndSystem(fmt.Sprintf("加载配置失败: %v", err))
	}

	// 解析GitHub URL
	parsedURL, _ := url.Parse("https://github.com/potatoone")

	// 将作者信息和GitHub链接放在一行
	authorContainer := container.NewHBox(
		widget.NewLabel("作者: onepotato"),
		widget.NewHyperlink("GitHub: https://github.com/potatoone", parsedURL),
	)

	var autoStartCheck *widget.Check
	var autoStartTaskCheck *widget.Check
	var lockUICheck *widget.Check // 添加界面锁定复选框变量

	// 添加开机自启动选项
	autoStartCheck = widget.NewCheck("开机自动启动", func(checked bool) {
		if checked {
			err := utils.EnableAutoStart()
			if err != nil {
				errMsg := fmt.Sprintf("设置开机自启动失败: %v", err)
				dialog.ShowError(fmt.Errorf("设置开机自启动失败: %v", err), win) // 修复：使用常量格式字符串
				aboutLogToUIAndSystem(errMsg)
				autoStartCheck.SetChecked(false)
				return
			}

			// 更新配置文件
			if config != nil {
				config.AutoStart = "true"
				if err := SaveConfig("config.json", config); err != nil {
					errMsg := fmt.Sprintf("保存配置失败: %v", err)
					dialog.ShowError(fmt.Errorf("保存配置失败: %v", err), win) // 修复：使用常量格式字符串
					aboutLogToUIAndSystem(errMsg)
				} else {
					aboutLogToUIAndSystem("开机自启动已启用")
				}
			}
		} else {
			err := utils.DisableAutoStart()
			if err != nil {
				errMsg := fmt.Sprintf("取消开机自启动失败: %v", err)
				dialog.ShowError(fmt.Errorf("取消开机自启动失败: %v", err), win) // 修复：使用常量格式字符串
				aboutLogToUIAndSystem(errMsg)
				autoStartCheck.SetChecked(true)
				return
			}

			// 更新配置文件
			if config != nil {
				config.AutoStart = "false"
				if err := SaveConfig("config.json", config); err != nil {
					errMsg := fmt.Sprintf("保存配置失败: %v", err)
					dialog.ShowError(fmt.Errorf("保存配置失败: %v", err), win) // 修复：使用常量格式字符串
					aboutLogToUIAndSystem(errMsg)
				} else {
					aboutLogToUIAndSystem("开机自启动已禁用")
				}
			}
		}
	})

	// 添加"程序启动自动执行任务"复选框（与开机自启完全独立）
	autoStartTaskCheck = widget.NewCheck("程序启动时自动执行任务", func(checked bool) {
		if config != nil {
			// 更新配置文件中的 autostart_auto 字段
			config.AutoStartAutoTask = fmt.Sprintf("%v", checked)
			if err := SaveConfig("config.json", config); err != nil {
				errMsg := fmt.Sprintf("保存配置失败: %v", err)
				dialog.ShowError(fmt.Errorf("保存配置失败: %v", err), win) // 修复：使用常量格式字符串
				aboutLogToUIAndSystem(errMsg)
				// 如果保存失败，恢复复选框状态
				autoStartTaskCheck.SetChecked(!checked)
				return
			}

			// 记录配置更改状态
			if checked {
				aboutLogToUIAndSystem("程序启动自动执行任务已启用")
			} else {
				aboutLogToUIAndSystem("程序启动自动执行任务已禁用")
			}
		}
	})

	// 添加"开启界面锁定"复选框
	lockUICheck = widget.NewCheck("开启界面锁定", func(checked bool) {
		if config != nil {
			// 更新配置文件中的 lockui 字段
			config.LockUI = fmt.Sprintf("%v", checked)
			if err := SaveConfig("config.json", config); err != nil {
				errMsg := fmt.Sprintf("保存配置失败: %v", err)
				dialog.ShowError(fmt.Errorf("保存配置失败: %v", err), win)
				aboutLogToUIAndSystem(errMsg)
				// 如果保存失败，恢复复选框状态
				lockUICheck.SetChecked(!checked)
				return
			}

			// 记录配置更改状态
			if checked {
				aboutLogToUIAndSystem("界面锁定已启用")
			} else {
				aboutLogToUIAndSystem("界面锁定已禁用")
			}
		}
	})

	// 设置复选框初始状态
	autoStartCheck.SetChecked(utils.IsAutoStartEnabled())
	if config != nil {
		autoStartTaskCheck.SetChecked(config.AutoStartAutoTask == "true")
		// 设置界面锁定复选框初始状态
		lockUICheck.SetChecked(config.LockUI == "true")
	}

	// 创建水平布局，三个复选框在同一行
	checkboxesContainer := container.NewHBox(
		autoStartCheck,
		widget.NewSeparator(), // 添加分隔符
		autoStartTaskCheck,
		widget.NewSeparator(), // 添加分隔符
		lockUICheck,           // 添加界面锁定复选框
	)

	aboutCard := widget.NewCard(
		"版本 v1.7.3",
		"应用功能：复制、压缩指定路径的图片，上传至 Minio，根据文件名查询API1，将文件 OSS 链接推送至 API2",
		container.NewVBox(
			authorContainer, // 使用放在一行的作者信息
			widget.NewSeparator(),
			checkboxesContainer,
		),
	)

	cleanSettingsCard := createCleanSettingsUI(win)

	return container.NewVBox(
		cleanSettingsCard,
		aboutCard,
	)
}
