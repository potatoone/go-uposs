package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-uposs/utils"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// heartbeatState 心跳运行状态管理
type heartbeatState struct {
	mu      sync.Mutex
	stopCh  chan struct{}
	running bool
}

// 全局心跳状态（程序启动时自动启动，UI 可控制）
var (
	globalHbState   = &heartbeatState{}
	monitorLogEntry *widget.Entry // UI 心跳日志框，供全局心跳写入
	pendingHbLogs   []string      // UI 建立前缓存的心跳日志，待 UI 可用后一次性显示
)

// maxHeartbeatLogLines 心跳日志在 UI 中最多保留的行数
const maxHeartbeatLogLines = 10

// updateHeartbeatLog 将心跳日志写入 UI 日志框（最多保留 maxHeartbeatLogLines 行，不写入系统日志文件）
func updateHeartbeatLog(message string) {
	// UI 尚未建立时，缓存日志，待 UI 可用后一次性显示
	if monitorLogEntry == nil {
		pendingHbLogs = append(pendingHbLogs, message)
		if len(pendingHbLogs) > maxHeartbeatLogLines {
			pendingHbLogs = pendingHbLogs[len(pendingHbLogs)-maxHeartbeatLogLines:]
		}
		return
	}

	fyne.Do(func() {
		// 构建带时间戳的日志消息
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fullMessage := fmt.Sprintf("%s [监控] %s", timestamp, message)

		// 新日志插入到最前面（新日志在上）
		currentText := monitorLogEntry.Text
		if currentText != "" {
			currentText = fullMessage + "\n" + currentText
		} else {
			currentText = fullMessage
		}

		// 最多保留 maxHeartbeatLogLines 行
		lines := strings.Split(currentText, "\n")
		if len(lines) > maxHeartbeatLogLines {
			lines = lines[:maxHeartbeatLogLines]
		}
		monitorLogEntry.SetText(strings.Join(lines, "\n"))
	})
}

// flushPendingHbLogs UI 建立后，将缓存的启动日志一次性写入 UI
func flushPendingHbLogs() {
	if monitorLogEntry == nil || len(pendingHbLogs) == 0 {
		return
	}
	logs := pendingHbLogs
	pendingHbLogs = nil
	// 按时间顺序写入（缓存是正序，UI 新日志在上，因此倒序插入保持时间顺序）
	for i := len(logs) - 1; i >= 0; i-- {
		updateHeartbeatLog(logs[i])
	}
}

// logHeartbeat 写入心跳日志（仅 UI 显示，最多 10 条，不写文件日志）
func logHeartbeat(message string) {
	updateHeartbeatLog(message)
}

// sendHeartbeatOnceUDP 通过 UDP 发送一次心跳并写日志
func sendHeartbeatOnceUDP(serverAddr, machineCode string) {
	info := utils.CollectSystemInfo()
	err := utils.SendHeartbeatUDP(serverAddr, machineCode, info)
	if err != nil {
		logHeartbeat(fmt.Sprintf("心跳失败: %v", err))
		return
	}
	logHeartbeat(fmt.Sprintf("心跳已发送 (UDP) -> %s", utils.StripURLScheme(serverAddr)))
}

// StartHeartbeatLoop 启动全局 UDP 心跳循环（程序启动时自动调用）
func StartHeartbeatLoop(config *Config, logText *widget.Entry) {
	if logText != nil {
		monitorLogEntry = logText
	}

	url := config.HeartbeatURL
	if url == "" {
		logHeartbeat("未配置心跳地址，自动心跳未启动")
		return
	}

	// 解析心跳间隔，默认 60 秒
	interval := 60
	if n, err := strconv.Atoi(config.HeartbeatInterval); err == nil && n > 0 {
		interval = n
	}

	if globalHbState.isRunning() {
		logHeartbeat("心跳已在运行中")
		return
	}

	stopCh := make(chan struct{})
	globalHbState.start(stopCh)

	logHeartbeat(fmt.Sprintf("自动心跳已启动 (UDP)，间隔 %d 秒，目标: %s", interval, utils.StripURLScheme(url)))

	go func() {
		// 启动后立即发送一次
		sendHeartbeatOnceUDP(url, config.MachineCode)

		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				logHeartbeat("自动心跳已停止")
				return
			case <-ticker.C:
				sendHeartbeatOnceUDP(url, config.MachineCode)
			}
		}
	}()
}

// isRunning 心跳是否在运行
func (h *heartbeatState) isRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// start 启动心跳循环
func (h *heartbeatState) start(stopCh chan struct{}) {
	h.mu.Lock()
	h.stopCh = stopCh
	h.running = true
	h.mu.Unlock()
}

// stop 停止心跳循环
func (h *heartbeatState) stop() {
	h.mu.Lock()
	if h.stopCh != nil && h.running {
		close(h.stopCh)
	}
	h.running = false
	h.mu.Unlock()
}

// createMonitorUI 创建监控界面 UI
func createMonitorUI(config *Config, myWindow fyne.Window) fyne.CanvasObject {
	// ---------- 系统信息显示 ----------
	sysInfoText := widget.NewMultiLineEntry()
	sysInfoText.SetMinRowsVisible(14)
	sysInfoText.Disable() // 只读

	// 采集时间标签
	updateTimeLabel := widget.NewLabel("")

	// 刷新系统信息
	refreshSysInfo := func() {
		go func() {
			info := utils.CollectSystemInfo()
			fyne.Do(func() {
				sysInfoText.SetText(utils.FormatSystemInfo(info))
				updateTimeLabel.SetText("采集时间: " + info.Timestamp)
			})
		}()
	}

	// 刷新按钮
	refreshButton := widget.NewButton("刷新系统信息", refreshSysInfo)

	// ---------- 心跳配置 ----------
	heartbeatURLEntry := widget.NewEntry()
	heartbeatURLEntry.SetText(config.HeartbeatURL)
	heartbeatURLEntry.SetPlaceHolder("心跳服务器地址（UDP），如 127.0.0.1:2200")

	heartbeatIntervalEntry := widget.NewEntry()
	if config.HeartbeatInterval == "" {
		config.HeartbeatInterval = "60"
	}
	heartbeatIntervalEntry.SetText(config.HeartbeatInterval)
	heartbeatIntervalEntry.SetPlaceHolder("心跳间隔（秒）")

	// ---------- 心跳日志 ----------
	heartbeatLogText := widget.NewMultiLineEntry()
	heartbeatLogText.SetMinRowsVisible(8)

	// 关联 UI 日志框到全局心跳日志
	monitorLogEntry = heartbeatLogText

	// 将启动阶段缓存的心跳日志一次性写入 UI
	flushPendingHbLogs()

	// 保存心跳配置
	saveHeartbeatConfig := func() {
		config.HeartbeatURL = heartbeatURLEntry.Text
		config.HeartbeatInterval = heartbeatIntervalEntry.Text
		if err := SaveConfig("config.json", config); err != nil {
			logHeartbeat(fmt.Sprintf("保存心跳配置失败: %v", err))
		} else {
			logHeartbeat("心跳配置已保存")
		}
	}

	// 发送一次心跳（UDP）
	sendOneHeartbeat := func() {
		url := heartbeatURLEntry.Text
		if url == "" {
			logHeartbeat("请先填写心跳服务器地址")
			return
		}
		logHeartbeat("正在发送心跳...")
		go sendHeartbeatOnceUDP(url, config.MachineCode)
	}

	// 开始/停止心跳切换按钮（先声明变量，以便回调中引用自身）
	var startStopButton *widget.Button
	startStopButton = widget.NewButton("开始心跳", func() {
		// 已运行 -> 停止
		if globalHbState.isRunning() {
			globalHbState.stop()
			startStopButton.SetText("开始心跳")
			logHeartbeat("已请求停止心跳")
			return
		}

		// 未运行 -> 开始
		url := heartbeatURLEntry.Text
		if url == "" {
			logHeartbeat("请先填写心跳服务器地址")
			return
		}

		// 解析心跳间隔
		intervalSec, err := strconv.Atoi(heartbeatIntervalEntry.Text)
		if err != nil || intervalSec <= 0 {
			logHeartbeat("心跳间隔无效，请填写正整数（秒）")
			return
		}

		// 保存配置
		saveHeartbeatConfig()

		// 启动全局心跳循环
		StartHeartbeatLoop(config, heartbeatLogText)
		startStopButton.SetText("停止心跳")
	})

	// 若程序启动时自动心跳已在运行，同步按钮状态为"停止心跳"
	if globalHbState.isRunning() {
		startStopButton.SetText("停止心跳")
	}

	// ---------- 布局 ----------
	// 顶部：刷新按钮 + 采集时间
	topBar := container.NewHBox(
		refreshButton,
		updateTimeLabel,
	)

	// 心跳配置区
	labelWidth := 100
	entryWidth := 360

	heartbeatURLContainer := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), widget.NewLabel("心跳地址:")),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), utils.LEBHeight), heartbeatURLEntry),
	)

	heartbeatIntervalContainer := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), widget.NewLabel("心跳间隔(秒):")),
		container.NewGridWrap(fyne.NewSize(float32(entryWidth), utils.LEBHeight), heartbeatIntervalEntry),
	)

	// 按钮区
	saveHBBtn := widget.NewButton("保存配置", saveHeartbeatConfig)
	sendOnceBtn := widget.NewButton("发送一次", sendOneHeartbeat)

	heartbeatButtons := container.NewHBox(
		startStopButton,
		saveHBBtn,
		sendOnceBtn,
	)

	heartbeatConfigBox := container.NewVBox(
		heartbeatURLContainer,
		heartbeatIntervalContainer,
		heartbeatButtons,
	)

	// 心跳配置卡片
	heartbeatConfigCard := container.NewVBox(
		widget.NewLabelWithStyle("心跳配置", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		heartbeatConfigBox,
	)

	// 系统信息卡片
	sysInfoCard := container.NewVBox(
		widget.NewLabelWithStyle("系统信息", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		sysInfoText,
	)

	// 心跳日志卡片
	heartbeatLogCard := container.NewVBox(
		widget.NewLabelWithStyle("心跳日志", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		heartbeatLogText,
	)

	// 合并布局：左列系统信息，右列心跳配置+日志
	leftColumn := container.NewVBox(
		container.NewPadded(sysInfoCard),
	)

	rightColumn := container.NewVBox(
		container.NewPadded(heartbeatConfigCard),
		container.NewPadded(heartbeatLogCard),
	)

	mainContent := container.NewHSplit(leftColumn, rightColumn)
	mainContent.SetOffset(0.55)

	monitorUI := container.NewBorder(
		topBar, // top
		nil,    // bottom
		nil,    // left
		nil,    // right
		mainContent,
	)

	// 界面加载时自动刷新一次系统信息
	refreshSysInfo()

	return monitorUI
}
