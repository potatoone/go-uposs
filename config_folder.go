package main

import (
	"fmt"
	"go-uposs/utils"
	"os"
	"runtime"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	fdlabelWidth = 140 // 标签固定宽度
	fdunitWidth  = 60  // 单位提示固定宽度

	perflabelWidth = 110 // 性能参数标签宽度（双列布局）
	perfunitWidth  = 55  // 性能参数范围提示宽度（双列布局）
)

var (
	remoteFolderEntry        = widget.NewEntry()          // 远端路径输入框
	localFolderEntry         = widget.NewEntry()          // 本地文件夹路径输入框
	ioBufferEntry            = widget.NewEntry()          // 缓冲区大小输入框
	cpuThreadsEntry          = widget.NewEntry()          // CPU 线程数输入框
	copyConcurrencyEntry     = widget.NewEntry()          // 复制并发数输入框
	compressConcurrencyEntry = widget.NewEntry()          // 压缩并发数输入框
	uploadConcurrencyEntry   = widget.NewEntry()          // 上传并发数输入框
	folderLogText            = widget.NewMultiLineEntry() // 用于显示日志信息
)

// 创建一个标签和输入框并排的组件，输入框自动拉伸铺满剩余宽度
func fdlabeledEntry(labelText string, entry *widget.Entry) fyne.CanvasObject {
	// 创建标签，并设置固定高度
	label := widget.NewLabelWithStyle(labelText, fyne.TextAlignLeading, fyne.TextStyle{})
	labelContainer := container.NewGridWrap(fyne.NewSize(fdlabelWidth, utils.LEBHeight), label) // 设置标签的宽度和高度

	return container.NewBorder(nil, nil, labelContainer, nil, entry)
}

// 创建带右侧单位/提示的标签与输入框组件，输入框自动拉伸铺满剩余宽度
func fdlabeledEntryWithTail(labelText string, entry *widget.Entry, tail fyne.CanvasObject, tailWidth float32) fyne.CanvasObject {
	label := widget.NewLabelWithStyle(labelText, fyne.TextAlignLeading, fyne.TextStyle{})
	labelContainer := container.NewGridWrap(fyne.NewSize(fdlabelWidth, utils.LEBHeight), label)
	tailContainer := container.NewGridWrap(fyne.NewSize(tailWidth, utils.LEBHeight), tail)

	return container.NewBorder(nil, nil, labelContainer, tailContainer, entry)
}

// 创建双列布局中使用的紧凑标签+输入框+范围提示组件，输入框自动拉伸
func perfLabeledEntry(labelText string, entry *widget.Entry, hint string) fyne.CanvasObject {
	label := widget.NewLabelWithStyle(labelText, fyne.TextAlignLeading, fyne.TextStyle{})
	labelContainer := container.NewGridWrap(fyne.NewSize(perflabelWidth, utils.LEBHeight), label)

	hintContainer := container.NewGridWrap(fyne.NewSize(perfunitWidth, utils.LEBHeight), widget.NewLabel(hint))

	return container.NewBorder(nil, nil, labelContainer, hintContainer, entry)
}

// 创建文件夹配置 UI
func createFolderConfigUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	// 初始化 remoteFolderEntry、localFolderEntry、ioBufferEntry 内容
	remoteFolderEntry.SetText(config.RemoteFolder)
	localFolderEntry.SetText(config.LocalFolder)
	ioBufferEntry.SetText(strconv.Itoa(config.IOBuffer / 1024)) // 将字节转换为KB
	cpuThreadsEntry.SetText(strconv.Itoa(config.CPUThreads))
	copyConcurrencyEntry.SetText(strconv.Itoa(config.CopyConcurrency))
	compressConcurrencyEntry.SetText(strconv.Itoa(config.CompressConcurrency))
	uploadConcurrencyEntry.SetText(strconv.Itoa(config.UploadConcurrency))

	// 清空日志框
	folderLogText.SetText("")
	folderLogText.SetMinRowsVisible(11) // 日志文本框显示行数，低于其他页面的 18 行以降低高度

	// 解析并校验并发数输入框，非法输入时记录日志
	parseConcurrency := func(entry *widget.Entry, name string) (int, bool) {
		value, err := strconv.Atoi(strings.TrimSpace(entry.Text))
		if err != nil || value < 1 || value > maxFileConcurrency {
			updateLog(folderLogText, "[文件夹配置]",
				fmt.Sprintf("无效的%s，请输入 1-%d 之间的整数", name, maxFileConcurrency))
			return 0, false
		}
		return value, true
	}

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

				// 获取用户输入的 CPU 线程数
				cpuThreads, err := strconv.Atoi(strings.TrimSpace(cpuThreadsEntry.Text))
				if err != nil || cpuThreads < 1 || cpuThreads > runtime.NumCPU() {
					updateLog(folderLogText, "[文件夹配置]",
						fmt.Sprintf("无效的 CPU 线程数，请输入 1-%d 之间的整数", runtime.NumCPU()))
					return
				}
				config.CPUThreads = cpuThreads

				// 获取用户输入的复制并发数
				copyConcurrency, ok := parseConcurrency(copyConcurrencyEntry, "复制并发数")
				if !ok {
					return
				}
				config.CopyConcurrency = copyConcurrency

				// 获取用户输入的压缩并发数
				compressConcurrency, ok := parseConcurrency(compressConcurrencyEntry, "压缩并发数")
				if !ok {
					return
				}
				config.CompressConcurrency = compressConcurrency

				// 获取用户输入的上传并发数
				uploadConcurrency, ok := parseConcurrency(uploadConcurrencyEntry, "上传并发数")
				if !ok {
					return
				}
				config.UploadConcurrency = uploadConcurrency

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
					// CPU 线程占用立即生效
					config.ApplyRuntimeLimits()
					// 配置保存成功，更新日志
					updateLog(folderLogText, "[文件夹配置]",
						fmt.Sprintf("配置保存成功！CPU 线程数: %d, 复制并发数: %d, 压缩并发数: %d, 上传并发数: %d",
							config.CPUThreads, config.CopyConcurrency, config.CompressConcurrency, config.UploadConcurrency))
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
	ioBufferContainer := fdlabeledEntryWithTail("IO Buffer:", ioBufferEntry, widget.NewLabel("KB"), fdunitWidth)
	cpuThreadsContainer := perfLabeledEntry("CPU 线程数:", cpuThreadsEntry, fmt.Sprintf("1-%d", runtime.NumCPU()))
	copyConcurrencyContainer := perfLabeledEntry("复制并发数:", copyConcurrencyEntry, fmt.Sprintf("1-%d", maxFileConcurrency))
	compressConcurrencyContainer := perfLabeledEntry("压缩并发数:", compressConcurrencyEntry, fmt.Sprintf("1-%d", maxFileConcurrency))
	uploadConcurrencyContainer := perfLabeledEntry("上传并发数:", uploadConcurrencyEntry, fmt.Sprintf("1-%d", maxFileConcurrency))

	// 性能参数使用双列布局，节省纵向空间
	perfGrid := container.NewGridWithColumns(2,
		cpuThreadsContainer, copyConcurrencyContainer,
		compressConcurrencyContainer, uploadConcurrencyContainer,
	)

	// 将输入框上下布局
	inputContainer := container.NewVBox(remoteFolderContainer, localFolderContainer, ioBufferContainer, perfGrid)

	// 创建按钮容器，按钮上下排列，并设置按钮的尺寸
	buttonContainer := container.NewVBox(
		container.NewGridWrap(fyne.NewSize(150, 50), saveButton),
		container.NewGridWrap(fyne.NewSize(150, 50), scanButton),
	)

	// 将输入框和按钮容器左右布局，并确保对齐
	inputAndButtonContainer := container.NewBorder(nil, nil, nil, buttonContainer, inputContainer)

	// 记录载入界面信息到系统日志
	SysLogToFile(fmt.Sprintf("[文件夹配置] 配置已载入，Remote=%s, Local=%s, IOBuffer=%dKB, CPU线程数=%d, 复制并发数=%d, 压缩并发数=%d, 上传并发数=%d",
		config.RemoteFolder, config.LocalFolder, config.IOBuffer/1024, config.CPUThreads,
		config.CopyConcurrency, config.CompressConcurrency, config.UploadConcurrency))

	// 使用内边距包裹，日志文本框铺满剩余的整页空间
	return container.NewPadded(container.NewBorder(
		inputAndButtonContainer, // top
		nil,                     // bottom
		nil,                     // left
		nil,                     // right
		folderLogText,           // center
	))
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
