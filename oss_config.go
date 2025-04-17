package main

import (
	"fmt"
	"go-uposs/utils"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	logText = widget.NewMultiLineEntry() // 用于显示日志信息

	// 配置字段的文本框
	machineCodeEntry     = widget.NewEntry()
	bucketNameEntry      = widget.NewEntry() // 添加存储桶名称输入框
	endpointEntry        = widget.NewEntry()
	publicUrlEntry       = widget.NewEntry()         // 公共URL输入框
	accessKeyIDEntry     = widget.NewPasswordEntry() // 隐藏输入内容
	secretAccessKeyEntry = widget.NewPasswordEntry() // 隐藏输入内容

	// UseSSL 复选框
	useSSLCheck = widget.NewCheck("使用 SSL", nil)
)

const (
	labelWidth = 150 // 标签固定宽度
	entryWidth = 460 // 文本框固定宽度
)

// 创建一个标签和输入框并排的组件
func labeledEntry(labelText string, entry *widget.Entry) fyne.CanvasObject {
	label := widget.NewLabelWithStyle(labelText, fyne.TextAlignLeading, fyne.TextStyle{})
	labelContainer := container.NewGridWrap(fyne.NewSize(labelWidth, utils.LEBHeight), label)

	// 固定文本框宽度
	entryContainer := container.NewGridWrap(fyne.NewSize(entryWidth, utils.LEBHeight), entry)

	return container.NewHBox(labelContainer, entryContainer)
}

// logToUIAndSystem 添加日志到UI和系统日志
func logToUIAndSystem(message string) {
	// 获取当前时间
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// 格式化消息，包含时间戳
	formattedMessage := fmt.Sprintf("%s %s", currentTime, message)

	// 添加到UI，新日志在顶部
	logMutex.Lock()
	logText.SetText(formattedMessage + "\n" + logText.Text)
	logMutex.Unlock()

	// 添加到系统日志
	SysLogToFile(fmt.Sprintf("[OSS配置] %s", message))
}

// saveConfig 保存配置
func saveConfig() {
	// 先加载现有配置
	config, err := LoadConfig("config.json") // 使用 utils.DataPath

	if err != nil {
		logToUIAndSystem(fmt.Sprintf("加载配置失败: %s", err.Error()))
		return
	}

	// 更新配置
	config.MachineCode = machineCodeEntry.Text
	config.BucketName = bucketNameEntry.Text // 保存存储桶名称
	config.Endpoint = endpointEntry.Text
	config.PublicUrl = publicUrlEntry.Text // 保存公共URL
	config.AccessKeyID = accessKeyIDEntry.Text
	config.SecretAccessKey = secretAccessKeyEntry.Text
	config.UseSSL = useSSLCheck.Checked

	// 保存配置
	if err := SaveConfig("config.json", config); err != nil {
		logToUIAndSystem(fmt.Sprintf("保存配置失败: %s", err.Error()))
	} else {
		logToUIAndSystem("配置保存成功！")
	}
}

// refreshConfig 刷新配置
func refreshConfig() {
	config, err := LoadConfig("config.json") // 使用 utils.DataPath

	if err != nil {
		logToUIAndSystem(fmt.Sprintf("加载配置失败: %s", err.Error()))
		return
	}

	// 更新文本框内容
	machineCodeEntry.SetText(config.MachineCode)
	bucketNameEntry.SetText(config.BucketName) // 刷新存储桶名称
	endpointEntry.SetText(config.Endpoint)
	publicUrlEntry.SetText(config.PublicUrl) // 刷新公共URL
	accessKeyIDEntry.SetText(config.AccessKeyID)
	secretAccessKeyEntry.SetText(config.SecretAccessKey)
	useSSLCheck.SetChecked(config.UseSSL)

	logToUIAndSystem("配置已刷新！")
}

// CreateUI 创建 UI 界面
func CreateUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	// 设置日志文本框
	logText.SetMinRowsVisible(11)
	logText.SetText("") // 清空初始内容

	// 初始化文本框内容
	machineCodeEntry.SetText(config.MachineCode)
	bucketNameEntry.SetText(config.BucketName) // 初始化存储桶名称
	endpointEntry.SetText(config.Endpoint)
	publicUrlEntry.SetText(config.PublicUrl) // 初始化公共URL
	accessKeyIDEntry.SetText(config.AccessKeyID)
	secretAccessKeyEntry.SetText(config.SecretAccessKey)
	useSSLCheck.SetChecked(config.UseSSL)

	// 创建按钮
	saveButton := widget.NewButton("保存配置", func() {
		dialog.ShowConfirm("确认保存", "你确定要保存配置吗？", func(confirm bool) {
			if confirm {
				saveConfig()
			}
		}, myWindow)
	})

	testButton := widget.NewButton("测试连接", func() {
		logToUIAndSystem("测试连接中...")
		useSSL := useSSLCheck.Checked
		result, err := TestMinioConnection(useSSL)
		if err != nil {
			logToUIAndSystem(fmt.Sprintf("错误: %s", err.Error()))
		} else {
			logToUIAndSystem(result)
		}
	})

	refreshButton := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		refreshConfig()
	})

	// 创建按钮容器，按钮上下排列，并设置按钮的尺寸
	buttonContainer := container.NewVBox(
		container.NewGridWrap(fyne.NewSize(140, utils.LEBHeight), useSSLCheck),
		container.NewGridWrap(fyne.NewSize(140, utils.LEBHeight), refreshButton),
		container.NewGridWrap(fyne.NewSize(140, utils.LEBHeight), saveButton),
		container.NewGridWrap(fyne.NewSize(140, utils.LEBHeight), testButton),
	)

	// 创建配置文本框容器
	configContainer := container.NewVBox(
		labeledEntry("Machine Code:", machineCodeEntry),
		labeledEntry("Bucket Name:", bucketNameEntry), // 添加存储桶名称字段
		labeledEntry("Endpoint:", endpointEntry),
		labeledEntry("Public URL:", publicUrlEntry), // 添加公共URL字段
		labeledEntry("Access Key ID:", accessKeyIDEntry),
		labeledEntry("Secret Access Key:", secretAccessKeyEntry),
	)

	// 将配置文本框容器和按钮容器水平排列
	mainContainer := container.NewBorder(nil, nil, nil, buttonContainer, configContainer)

	// 记录载入界面信息到系统日志
	SysLogToFile(fmt.Sprintf("[OSS配置] 配置已载入，MachineCode=%s, BucketName=%s, Endpoint=%s, UseSSL=%t",
		config.MachineCode, config.BucketName, config.Endpoint, config.UseSSL))

	// 布局
	return container.NewVBox(
		mainContainer,
		logText,
	)
}
