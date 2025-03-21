package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// TestAPIHTTP 使用 HTTP 请求测试 API 是否可访问
func TestAPIHTTP(apiURL string) (int, time.Duration, error) {
	// 创建一个带有超时的 HTTP 客户端
	client := http.Client{
		Timeout: 10 * time.Second, // 设置 10 秒超时
	}

	// 确保 URL 有正确的协议前缀
	if !strings.HasPrefix(apiURL, "http://") && !strings.HasPrefix(apiURL, "https://") {
		apiURL = "https://" + apiURL
	}

	// 创建请求
	req, err := http.NewRequest("HEAD", apiURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加常用的请求头
	req.Header.Add("User-Agent", "Mozilla/5.0 GoUpOSS Client")

	// 发送请求
	startTime := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return 0, 0, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 返回状态码和响应时间
	return resp.StatusCode, duration, nil
}

// FormatAPITestResult 格式化 API 测试结果
func FormatAPITestResult(statusCode int, duration time.Duration) string {
	if statusCode >= 200 && statusCode < 400 {
		return fmt.Sprintf("接口可用 (状态码: %d, 响应时间: %s)", statusCode, duration)
	} else {
		return fmt.Sprintf("接口状态码: %d, 响应时间: %s", statusCode, duration)
	}
}

// 创建 API 配置 UI
func createAPIConfigUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	// 创建 api1 和 api2 输入框
	api1Entry := widget.NewEntry() // 普通输入框，便于输入和查看URL
	api1Entry.SetText(config.API1)

	api2Entry := widget.NewEntry()
	api2Entry.SetText(config.API2)

	// 创建标签
	api1Label := widget.NewLabel("API 1:")
	api2Label := widget.NewLabel("API 2:")

	// 创建日志输出框
	apiLogText := widget.NewMultiLineEntry()
	apiLogText.SetMinRowsVisible(16)

	// 添加日志到 UI 和系统日志的函数
	logToUIAndSystem := func(message string) {
		// 获取当前时间
		currentTime := time.Now().Format("2006-01-02 15:04:05")

		// 格式化消息，包含时间戳
		formattedMessage := fmt.Sprintf("%s %s", currentTime, message)

		// 添加到 UI
		apiLogText.SetText(formattedMessage + "\n" + apiLogText.Text)

		// 添加到系统日志
		SysLogToFile(fmt.Sprintf("[API配置] %s", message))
	}

	// 创建保存按钮
	saveButton := widget.NewButton("保存配置", func() {
		dialog.ShowConfirm("确认保存", "确定要保存配置吗？", func(confirm bool) {
			if confirm {
				config.API1 = api1Entry.Text
				config.API2 = api2Entry.Text

				// 直接使用文件名，不构建路径
				if err := SaveConfig("config.json", config); err != nil {
					errorMsg := fmt.Sprintf("保存配置失败: %v", err)
					logToUIAndSystem(errorMsg)
					dialog.ShowInformation("保存失败", errorMsg, myWindow)
				} else {
					logToUIAndSystem("配置已成功保存")
				}
			}
		}, myWindow)
	})

	// 创建 API 测试按钮
	testButton := widget.NewButton("HTTP 测试", func() {
		// 测试 API 连接
		api1URL := api1Entry.Text
		api2URL := api2Entry.Text

		if api1URL == "" && api2URL == "" {
			logToUIAndSystem("请至少输入一个 API 地址")
			return
		}

		// 开始测试
		logToUIAndSystem("开始测试 API...")

		// 在后台线程中测试，避免阻塞 UI
		go func() {
			// 测试 API1
			if api1URL != "" {
				statusCode, duration, err := TestAPIHTTP(api1URL)
				if err != nil {
					logToUIAndSystem(fmt.Sprintf("API1: %v", err))
				} else {
					result := FormatAPITestResult(statusCode, duration)
					logToUIAndSystem(fmt.Sprintf("API1: %s", result))
				}
			}

			// 测试 API2
			if api2URL != "" {
				statusCode, duration, err := TestAPIHTTP(api2URL)
				if err != nil {
					logToUIAndSystem(fmt.Sprintf("API2: %v", err))
				} else {
					result := FormatAPITestResult(statusCode, duration)
					logToUIAndSystem(fmt.Sprintf("API2: %s", result))
				}
			}

			logToUIAndSystem("API 测试完成")
		}()
	})

	// 设置标签和输入框的宽度
	labelWidth := 70
	entryWidth := 500 // 减小输入框宽度，为右侧按钮腾出空间

	// 创建输入框的容器
	api1Container := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), api1Label.MinSize().Height), api1Label),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), api1Entry.MinSize().Height), api1Entry),
	)

	api2Container := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), api2Label.MinSize().Height), api2Label),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), api2Entry.MinSize().Height), api2Entry),
	)

	// 将保存和测试按钮放在垂直容器中，放在右侧
	buttonWidth := 150
	buttonHeight := 35

	// 设置按钮宽度和高度
	saveButtonContainer := container.NewGridWrap(
		fyne.NewSize(float32(buttonWidth), float32(buttonHeight)),
		saveButton,
	)

	testButtonContainer := container.NewGridWrap(
		fyne.NewSize(float32(buttonWidth), float32(buttonHeight)),
		testButton,
	)

	// 创建右侧按钮容器
	rightButtons := container.NewVBox(
		saveButtonContainer,
		testButtonContainer,
	)

	// 记录载入界面信息到系统日志
	SysLogToFile(fmt.Sprintf("[API配置] 配置已载入，API1长度:%d, API2长度:%d",
		len(config.API1), len(config.API2)))

	// 创建输入框容器
	inputsContainer := container.NewVBox(
		api1Container,
		api2Container,
	)

	// 创建顶部容器，将输入框容器和右侧按钮容器组合
	topContainer := container.NewBorder(
		nil,          // top
		nil,          // bottom
		nil,          // left
		rightButtons, // right
		inputsContainer,
	)

	// 创建 API 配置 UI 布局
	apiConfigUI := container.NewBorder(
		topContainer, // top
		nil,          // bottom
		nil,          // left
		nil,          // right
		apiLogText,   // center
	)

	return apiConfigUI
}
