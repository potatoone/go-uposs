package main

import (
	"fmt"
	"go-uposs/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	labelWidth = 150 // 标签固定宽度
	entryWidth = 460 // 文本框固定宽度
)

var ossLogText *widget.Entry

// 创建一个标签和输入框并排的组件
func labeledEntry(labelText string, entry *widget.Entry) fyne.CanvasObject {
	label := widget.NewLabelWithStyle(labelText, fyne.TextAlignLeading, fyne.TextStyle{})
	labelContainer := container.NewGridWrap(fyne.NewSize(labelWidth, utils.LEBHeight), label)
	entryContainer := container.NewGridWrap(fyne.NewSize(entryWidth, utils.LEBHeight), entry)
	return container.NewHBox(labelContainer, entryContainer)
}

// saveConfig 保存 OSS 配置
func saveConfig(machineCodeEntry, bucketNameEntry, endpointEntry, publicUrlEntry, accessKeyIDEntry, secretAccessKeyEntry *widget.Entry, useSSLCheck *widget.Check) {
	config, err := LoadConfig("config.json")
	if err != nil {
		updateLog(ossLogText, "[OSS配置]", fmt.Sprintf("加载配置失败: %s", err.Error()))
		return
	}

	config.MachineCode = machineCodeEntry.Text
	config.BucketName = bucketNameEntry.Text
	config.Endpoint = endpointEntry.Text
	config.PublicUrl = publicUrlEntry.Text
	config.AccessKeyID = accessKeyIDEntry.Text
	config.SecretAccessKey = secretAccessKeyEntry.Text
	config.UseSSL = useSSLCheck.Checked

	if err := SaveConfig("config.json", config); err != nil {
		updateLog(ossLogText, "[OSS配置]", fmt.Sprintf("保存配置失败: %s", err.Error()))
	} else {
		updateLog(ossLogText, "[OSS配置]", "配置保存成功！")
	}
}

// refreshConfig 刷新 OSS 配置
func refreshConfig(machineCodeEntry, bucketNameEntry, endpointEntry, publicUrlEntry, accessKeyIDEntry, secretAccessKeyEntry *widget.Entry, useSSLCheck *widget.Check) {
	config, err := LoadConfig("config.json")
	if err != nil {
		updateLog(ossLogText, "[OSS配置]", fmt.Sprintf("加载配置失败: %s", err.Error()))
		return
	}

	machineCodeEntry.SetText(config.MachineCode)
	bucketNameEntry.SetText(config.BucketName)
	endpointEntry.SetText(config.Endpoint)
	publicUrlEntry.SetText(config.PublicUrl)
	accessKeyIDEntry.SetText(config.AccessKeyID)
	secretAccessKeyEntry.SetText(config.SecretAccessKey)
	useSSLCheck.SetChecked(config.UseSSL)

	updateLog(ossLogText, "[OSS配置]", "配置已刷新！")
}

// CreateUI 创建 UI 界面
func CreateUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	ossLogText = widget.NewMultiLineEntry()
	ossLogText.SetMinRowsVisible(11)
	ossLogText.SetText("")

	machineCodeEntry := widget.NewEntry()
	bucketNameEntry := widget.NewEntry()
	endpointEntry := widget.NewEntry()
	publicUrlEntry := widget.NewEntry()
	accessKeyIDEntry := widget.NewPasswordEntry()
	secretAccessKeyEntry := widget.NewPasswordEntry()
	useSSLCheck := widget.NewCheck("使用 SSL", nil)

	machineCodeEntry.SetText(config.MachineCode)
	bucketNameEntry.SetText(config.BucketName)
	endpointEntry.SetText(config.Endpoint)
	publicUrlEntry.SetText(config.PublicUrl)
	accessKeyIDEntry.SetText(config.AccessKeyID)
	secretAccessKeyEntry.SetText(config.SecretAccessKey)
	useSSLCheck.SetChecked(config.UseSSL)

	saveButton := widget.NewButton("保存配置", func() {
		dialog.ShowConfirm("确认保存", "你确定要保存配置吗？", func(confirm bool) {
			if confirm {
				saveConfig(machineCodeEntry, bucketNameEntry, endpointEntry, publicUrlEntry, accessKeyIDEntry, secretAccessKeyEntry, useSSLCheck)
			}
		}, myWindow)
	})

	// 创建测试连接按钮
	testButton := widget.NewButton("测试连接", func() {
		updateLog(ossLogText, "[OSS配置]", "测试连接中...") // 使用统一的日志更新函数
		useSSL := useSSLCheck.Checked
		result, err := TestMinioConnection(useSSL)
		if err != nil {
			updateLog(ossLogText, "[OSS配置]", fmt.Sprintf("错误: %s", err.Error()))
		} else {
			updateLog(ossLogText, "[OSS配置]", result)
		}
	})

	refreshButton := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		refreshConfig(machineCodeEntry, bucketNameEntry, endpointEntry, publicUrlEntry, accessKeyIDEntry, secretAccessKeyEntry, useSSLCheck)
	})

	buttonContainer := container.NewVBox(
		container.NewGridWrap(fyne.NewSize(140, utils.LEBHeight), useSSLCheck),
		container.NewGridWrap(fyne.NewSize(140, utils.LEBHeight), refreshButton),
		container.NewGridWrap(fyne.NewSize(140, utils.LEBHeight), saveButton),
		container.NewGridWrap(fyne.NewSize(140, utils.LEBHeight), testButton),
	)

	configContainer := container.NewVBox(
		labeledEntry("Machine Code:", machineCodeEntry),
		labeledEntry("Bucket Name:", bucketNameEntry),
		labeledEntry("Endpoint:", endpointEntry),
		labeledEntry("Public URL:", publicUrlEntry),
		labeledEntry("Access Key ID:", accessKeyIDEntry),
		labeledEntry("Secret Access Key:", secretAccessKeyEntry),
	)

	mainContainer := container.NewBorder(nil, nil, nil, buttonContainer, configContainer)

	SysLogToFile(fmt.Sprintf("[OSS配置] 配置已载入，MachineCode=%s, BucketName=%s, Endpoint=%s, UseSSL=%t",
		config.MachineCode, config.BucketName, config.Endpoint, config.UseSSL))

	return container.NewVBox(
		mainContainer,
		ossLogText,
	)
}
