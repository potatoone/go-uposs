package main

import (
	"fmt"
	"go-uposs/utils"
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
	if statusCode >= 200 && statusCode < 500 {
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

	api1response1 := widget.NewEntry() // API1 有效响应 输入框
	api1response1.SetText(config.API1Response1)

	api1response2 := widget.NewEntry() // API1 无效响应 输入框
	api1response2.SetText(config.API1Response2)

	webhookEntry := widget.NewEntry()
	webhookEntry.SetText(config.WebhookURL)

	// 创建标签
	api1Label := widget.NewLabel("API 1:")
	api2Label := widget.NewLabel("API 2:")
	api1Response1Label := widget.NewLabel("API1 有效响应:")
	api1Response2Label := widget.NewLabel("API1 无效响应:")
	webhookLabel := widget.NewLabel("Webhook URL:")

	// 创建日志输出框
	apiLogText := widget.NewMultiLineEntry()
	apiLogText.SetMinRowsVisible(13)

	// 创建保存按钮
	saveButton := widget.NewButton("保存配置", func() {
		dialog.ShowConfirm("确认保存", "确定要保存配置吗？", func(confirm bool) {
			if confirm {
				config.API1 = api1Entry.Text
				config.API2 = api2Entry.Text
				config.API1Response1 = api1response1.Text
				config.API1Response2 = api1response2.Text
				config.WebhookURL = webhookEntry.Text

				// 假设你有一个 apiLogText 变量表示 API 配置那一栏的日志框
				if err := SaveConfig("config.json", config); err != nil {
					errorMsg := fmt.Sprintf("保存配置失败: %v", err)
					updateLog(apiLogText, "[API配置]", errorMsg)
					dialog.ShowInformation("保存失败", errorMsg, myWindow)
				} else {
					updateLog(apiLogText, "[API配置]", "配置已成功保存")
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
			updateLog(apiLogText, "[API配置]", "请至少输入一个 API 地址")
			return
		}

		// 开始测试
		updateLog(apiLogText, "[API配置]", "开始测试 API...")

		// 在后台线程中测试，避免阻塞 UI
		go func() {
			// 测试 API1
			if api1URL != "" {
				statusCode, duration, err := TestAPIHTTP(api1URL)
				if err != nil {
					updateLog(apiLogText, "[API配置]", fmt.Sprintf("API1: %v", err))
				} else {
					result := FormatAPITestResult(statusCode, duration)
					updateLog(apiLogText, "[API配置]", fmt.Sprintf("API1: %s", result))
				}
			}

			// 测试 API2
			if api2URL != "" {
				statusCode, duration, err := TestAPIHTTP(api2URL)
				if err != nil {
					updateLog(apiLogText, "[API配置]", fmt.Sprintf("API2: %v", err))
				} else {
					result := FormatAPITestResult(statusCode, duration)
					updateLog(apiLogText, "[API配置]", fmt.Sprintf("API2: %s", result))
				}
			}

			updateLog(apiLogText, "[API配置]", "API 测试完成")
		}()
	})

	// 设置标签和输入框的宽度
	labelWidth := 120
	entryWidth := 460 // 减小输入框宽度，为右侧按钮腾出空间

	// 创建输入框的容器
	api1Container := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), api1Label),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), utils.LEBHeight), api1Entry),
	)

	api2Container := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), api2Label),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), utils.LEBHeight), api2Entry),
	)

	api1Response1Container := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), api1Response1Label),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), utils.LEBHeight), api1response1),
	)

	api1Response2Container := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), api1Response2Label),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), utils.LEBHeight), api1response2),
	)

	webhookContainer := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), webhookLabel),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), utils.LEBHeight), webhookEntry),
	)

	// 将保存和测试按钮放在垂直容器中，放在右侧
	buttonWidth := 150
	buttonHeight := 32

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
		api1Response1Container,
		api1Response2Container,
		webhookContainer,
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
