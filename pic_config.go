package main

import (
	"fmt"
	"strconv"
	"time" // 添加时间包

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	piclabelWidth = 150 // 标签固定宽度
	picentryWidth = 330 // 文本框固定宽度
)

// 创建一个标签和输入框并排的组件，后面附加单位
func createLabeledEntryWithUnit(labelText string, entry *widget.Entry, unit string) fyne.CanvasObject {
	label := widget.NewLabelWithStyle(labelText, fyne.TextAlignLeading, fyne.TextStyle{})
	labelContainer := container.NewGridWrap(fyne.NewSize(piclabelWidth, label.MinSize().Height), label)

	// 固定文本框宽度
	entryContainer := container.NewGridWrap(fyne.NewSize(picentryWidth, entry.MinSize().Height), entry)

	// 创建单位标签
	unitLabel := widget.NewLabel(unit)
	unitLabelContainer := container.NewGridWrap(fyne.NewSize(50, unitLabel.MinSize().Height), unitLabel)

	// 将输入框和单位标签并排
	return container.NewHBox(labelContainer, entryContainer, unitLabelContainer)
}

// 创建图片配置界面的 UI
func createPicConfigUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	// 创建一个进度条，并初始化为配置中的压缩比率
	progress := widget.NewProgressBar()
	compress, err := strconv.Atoi(config.PicCompress) // 将配置中的压缩比率转换为整数
	if err == nil && compress >= 1 && compress <= 100 {
		progress.SetValue(float64(compress) / 100.0) // 设置进度条初始值
	} else {
		progress.SetValue(0) // 如果转换失败，设置为 0
	}

	// 创建压缩比率输入框
	compressInput := widget.NewEntry()
	compressInput.SetPlaceHolder("请输入压缩比率（1-100）") // 提示用户输入压缩比率
	compressInput.SetText(config.PicCompress)      // 设置默认值为配置文件中的压缩比率

	// 创建一个宽度输入框
	widthInput := widget.NewEntry()
	widthInput.SetPlaceHolder("请输入宽度")  // 提示用户输入宽度
	widthInput.SetText(config.PicWidth) // 设置默认值为配置文件中的宽度

	// 创建一个体积输入框
	sizeInput := widget.NewEntry()
	sizeInput.SetPlaceHolder("请输入体积（KB）")           // 提示用户输入体积
	sizeInput.SetText(strconv.Itoa(config.PicSize)) // 设置默认值为配置文件中的体积

	// 创建一个日志输出框（多行文本框）
	logOutput := widget.NewMultiLineEntry()
	logOutput.SetMinRowsVisible(14) // 设置日志文本框可见行数
	logOutput.SetText("")           // 确保初始文本为空，没有空行

	// 添加日志到UI和系统日志
	logToUIAndSystem := func(message string) {
		// 获取当前时间
		currentTime := time.Now().Format("2006-01-02 15:04:05")

		// 格式化消息，包含时间戳
		formattedMessage := fmt.Sprintf("%s %s", currentTime, message)

		// 添加到UI
		logOutput.SetText(formattedMessage + "\n" + logOutput.Text)

		// 添加到系统日志
		SysLogToFile(fmt.Sprintf("[图片配置] %s", message))
	}

	confirmButton := widget.NewButton("修改参数", func() {
		// 获取用户输入的宽度
		widthStr := widthInput.Text
		width, err := strconv.Atoi(widthStr)
		if err != nil || width < 1 || width > 10000 {
			// 如果输入无效，打印错误
			logToUIAndSystem("请输入有效的宽度（1-10000）！")
			return
		}

		// 获取用户输入的压缩比率
		compressStr := compressInput.Text
		compress, err := strconv.Atoi(compressStr)
		if err != nil || compress < 1 || compress > 100 {
			// 如果压缩比率无效，打印错误
			logToUIAndSystem("请输入有效的压缩比率（1-100）！")
			return
		}

		// 获取用户输入的体积
		sizeStr := sizeInput.Text
		size, err := strconv.Atoi(sizeStr)
		if err != nil || size < 1 {
			// 如果体积无效，打印错误
			logToUIAndSystem("请输入有效的体积（KB）！")
			return
		}

		// 弹出确认对话框
		dialog.ShowConfirm("确认保存", "你确定要保存配置吗？", func(confirmed bool) {
			if !confirmed {
				return
			}

			// 更新配置文件中的 pic_compress、pic_width 和 pic_size
			config.PicWidth = widthStr
			config.PicCompress = compressStr
			config.PicSize = size

			// 直接使用传入的 config 实例，不重新加载
			if err := SaveConfig("config.json", config); err != nil {
				errMsg := fmt.Sprintf("保存配置失败: %s", err.Error())
				logToUIAndSystem(errMsg)
				return
			}

			// 假设压缩比率与进度条值相关，1-100
			progress.SetValue(float64(compress) / 100.0) // 设置进度条值，范围为 0.0 到 1.0

			// 输出操作成功日志
			successMsg := fmt.Sprintf("压缩比率设置为: %d%%，宽度设置为: %d，过滤图片的大小设置为: %dKB",
				compress, width, size)
			logToUIAndSystem(successMsg)
		}, myWindow) // myWindow 是当前窗口的引用
	})

	// 将按钮放在一个容器中，并设置宽度和高度
	buttonContainer := container.NewGridWrap(fyne.NewSize(200, 75), confirmButton)

	// 使用 createLabeledEntryWithUnit 函数将标签和输入框组合成水平排列的组件
	compressBox := createLabeledEntryWithUnit("图片质量：", compressInput, "%")
	widthBox := createLabeledEntryWithUnit("图片宽度：", widthInput, "px")
	sizeBox := createLabeledEntryWithUnit("过滤大小：", sizeInput, "KB")

	// 记录载入界面信息到系统日志
	SysLogToFile(fmt.Sprintf("[图片配置] 配置已载入，压缩率=%s%%，宽度=%s，过滤大小=%dKB",
		config.PicCompress, config.PicWidth, config.PicSize))

	// 将控件放到垂直布局中，并将百分比条和按钮放在右边，文本框放在下面
	return container.NewBorder(
		nil,       // top
		logOutput, // bottom
		nil,       // left
		container.NewVBox(progress, buttonContainer), // right
		container.NewVBox(
			compressBox, // 压缩比率的标签和输入框
			widthBox,    // 宽度的标签和输入框
			sizeBox,     // 体积的标签和输入框
		),
	)
}
