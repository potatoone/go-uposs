package main

import (
	"fmt"
	"go-uposs/utils"
	"os"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	fdlabelWidth = 140 // 标签固定宽度
	fdentryWidth = 400 // 文本框固定宽度
)

var (
	remoteFolderEntry = widget.NewEntry()          // 远端路径输入框
	localFolderEntry  = widget.NewEntry()          // 本地文件夹路径输入框
	ioBufferEntry     = widget.NewEntry()          // 缓冲区大小输入框
	folderLogText     = widget.NewMultiLineEntry() // 用于显示日志信息
)

// 创建一个标签和输入框并排的组件
func fdlabeledEntry(labelText string, entry *widget.Entry) fyne.CanvasObject {
	// 创建标签，并设置固定高度
	label := widget.NewLabelWithStyle(labelText, fyne.TextAlignLeading, fyne.TextStyle{})
	labelContainer := container.NewGridWrap(fyne.NewSize(fdlabelWidth, utils.LEBHeight), label) // 设置标签的宽度和高度

	// 创建输入框，并设置固定高度
	entryContainer := container.NewGridWrap(fyne.NewSize(fdentryWidth, utils.LEBHeight), entry) // 设置输入框的宽度和高度

	return container.NewHBox(labelContainer, entryContainer)
}

// 创建文件夹配置 UI
func createFolderConfigUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	// 初始化 remoteFolderEntry、localFolderEntry、ioBufferEntry 内容
	remoteFolderEntry.SetText(config.RemoteFolder)
	localFolderEntry.SetText(config.LocalFolder)
	ioBufferEntry.SetText(strconv.Itoa(config.IOBuffer / 1024)) // 将字节转换为KB

	// 清空日志框
	folderLogText.SetText("")
	folderLogText.SetMinRowsVisible(18) // 设置日志显示的行数，保持与其他页面一致

	// 创建保存配置按钮
	saveButton := widget.NewButton("保存配置", func() {
		dialog.ShowConfirm("确认保存", "你确定要保存配置吗？", func(confirm bool) {
			if confirm {
				// 更新配置
				config.RemoteFolder = remoteFolderEntry.Text
				config.LocalFolder = localFolderEntry.Text

				// 获取用户输入的缓冲区大小
				ioBuffer, err := strconv.Atoi(ioBufferEntry.Text)
				if err != nil {
					updateLog(folderLogText, "[文件夹配置]", fmt.Sprintf("无效的缓冲区大小: %s", err.Error()))
					return
				}
				config.IOBuffer = ioBuffer * 1024 // 将KB转换为字节

				// 检查路径是否为空
				if config.RemoteFolder == "" || config.LocalFolder == "" {
					updateLog(folderLogText, "[文件夹配置]", "路径不能为空，请检查输入。")
					return
				}

				// 保存配置 - 直接使用文件名，不构建路径
				if err := SaveConfig("config.json", config); err != nil {
					// 保存失败，显示错误信息
					updateLog(folderLogText, "[文件夹配置]", fmt.Sprintf("保存配置失败: %s", err.Error()))
				} else {
					// 配置保存成功，更新日志
					updateLog(folderLogText, "[文件夹配置]", "配置保存成功！")
				}

			}
		}, myWindow)
	})

	// 创建扫描文件夹测试按钮
	scanButton := widget.NewButton("扫描文件夹测试", func() {
		// 获取当前输入框中的值
		remotePath := remoteFolderEntry.Text
		if remotePath == "" {
			updateLog(folderLogText, "[扫描文件夹]", "请输入远端文件夹路径")
			return
		}

		updateLog(folderLogText, "[扫描文件夹]", fmt.Sprintf("开始扫描文件夹: %s", remotePath))

		// 扫描 remote 目录下的所有文件夹名称
		folders, err := scanRemoteFolders(remotePath)
		if err != nil {
			updateLog(folderLogText, "[扫描文件夹]", fmt.Sprintf("扫描远端文件夹失败: %s", err.Error()))
			return
		}

		// 将文件夹名称追加到日志文本框中，名称逗号分隔不换行显示
		updateLog(folderLogText, "[扫描文件夹]", fmt.Sprintf("扫描完成，找到 %d 个文件夹: %s",
			len(folders), strings.Join(folders, ", ")))
	})

	// 创建 remoteFolderEntry 和 localFolderEntry 的标签和输入框，并将保存按钮放在右边
	remoteFolderContainer := fdlabeledEntry("Remote Folder:", remoteFolderEntry)
	localFolderContainer := fdlabeledEntry("Local Folder:", localFolderEntry)
	ioBufferContainer := container.NewHBox(
		fdlabeledEntry("IO Buffer:", ioBufferEntry),
		widget.NewLabel("KB"),
	)

	// 将输入框上下布局
	inputContainer := container.NewVBox(remoteFolderContainer, localFolderContainer, ioBufferContainer)

	// 创建按钮容器，按钮上下排列，并设置按钮的尺寸
	buttonContainer := container.NewVBox(
		container.NewGridWrap(fyne.NewSize(150, 50), saveButton),
		container.NewGridWrap(fyne.NewSize(150, 50), scanButton),
	)

	// 将输入框和按钮容器左右布局，并确保对齐
	inputAndButtonContainer := container.NewBorder(nil, nil, nil, buttonContainer, inputContainer)

	// 记录载入界面信息到系统日志
	SysLogToFile(fmt.Sprintf("[文件夹配置] 配置已载入，Remote=%s, Local=%s, IOBuffer=%dKB",
		config.RemoteFolder, config.LocalFolder, config.IOBuffer/1024))

	// 使用 container.NewVBox 将输入框、按钮和日志输出框垂直排列
	return container.NewVBox(inputAndButtonContainer, folderLogText)
}

// 扫描 remote 目录下的所有文件夹名称
func scanRemoteFolders(remoteFolder string) ([]string, error) {
	entries, err := os.ReadDir(remoteFolder)
	if err != nil {
		return nil, err
	}

	var folders []string
	for _, entry := range entries {
		if entry.IsDir() {
			folders = append(folders, entry.Name())
		}
	}

	return folders, nil
}
