package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-uposs/internal/utils"

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
	// 12 行 = 5 项主机/CPU 信息 + 内存 + 6 个磁盘盘符，正好铺满常见内容且不留下空白
	sysInfoText.SetMinRowsVisible(12)
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

	// ---------- 时间同步 ----------
	ntpServerEntry := widget.NewEntry()
	ntpServerEntry.SetText(config.NTPServer)
	ntpServerEntry.SetPlaceHolder("NTP 服务器，如 ntp.aliyun.com")

	ntpIntervalEntry := widget.NewEntry()
	ntpIntervalEntry.SetText(config.NTPInterval)
	ntpIntervalEntry.SetPlaceHolder("检测间隔（秒）")

	ntpThresholdEntry := widget.NewEntry()
	ntpThresholdEntry.SetText(config.NTPThreshold)
	ntpThresholdEntry.SetPlaceHolder("偏差告警阈值（秒）")

	// 时间同步状态标签：显示最近一次检测的网络时间与偏差
	timeSyncStatusLabel := widget.NewLabel("尚未检测")
	timeSyncStatusLabel.Wrapping = fyne.TextWrapWord

	globalTimeSync.mu.Lock()
	globalTimeSync.statusLabel = timeSyncStatusLabel
	globalTimeSync.mu.Unlock()

	// 若启动时已完成检测，直接显示结果
	if last, err := GetLastTimeSync(); !last.CheckedAt.IsZero() {
		if err != nil {
			timeSyncStatusLabel.SetText(fmt.Sprintf("检测失败: %v", err))
		} else {
			timeSyncStatusLabel.SetText(fmt.Sprintf("网络时间 %s　本机偏差 %+.3f 秒",
				last.RemoteTime.Format("15:04:05"), last.Offset.Seconds()))
		}
	}

	// 保存时间同步配置
	saveTimeSyncConfig := func() bool {
		intervalSec, err := strconv.Atoi(ntpIntervalEntry.Text)
		if err != nil || intervalSec <= 0 {
			logHeartbeat("检测间隔无效，请填写正整数（秒）")
			return false
		}
		thresholdSec, err := strconv.Atoi(ntpThresholdEntry.Text)
		if err != nil || thresholdSec <= 0 {
			logHeartbeat("偏差告警阈值无效，请填写正整数（秒）")
			return false
		}
		if ntpServerEntry.Text == "" {
			logHeartbeat("请先填写 NTP 服务器地址")
			return false
		}

		config.NTPServer = ntpServerEntry.Text
		config.NTPInterval = ntpIntervalEntry.Text
		config.NTPThreshold = ntpThresholdEntry.Text
		if err := SaveConfig("config.json", config); err != nil {
			logHeartbeat(fmt.Sprintf("保存时间同步配置失败: %v", err))
			return false
		}
		logHeartbeat("时间同步配置已保存")
		return true
	}

	// 立即检测一次网络时间
	checkTimeNow := func() {
		if ntpServerEntry.Text == "" {
			logHeartbeat("请先填写 NTP 服务器地址")
			return
		}
		config.NTPServer = ntpServerEntry.Text
		logHeartbeat(fmt.Sprintf("正在向 %s 查询网络时间...", config.NTPServer))
		go checkTimeOnce(config)
	}

	// ---------- 心跳日志 ----------
	heartbeatLogText := widget.NewMultiLineEntry()
	heartbeatLogText.SetMinRowsVisible(5)

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

	// 心跳配置区：输入框拉伸铺满剩余宽度
	labelWidth := 100

	heartbeatURLContainer := container.NewBorder(
		nil,
		nil,
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), widget.NewLabel("心跳地址:")),
		nil,
		heartbeatURLEntry,
	)

	heartbeatIntervalContainer := container.NewBorder(
		nil,
		nil,
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), widget.NewLabel("心跳间隔(秒):")),
		nil,
		heartbeatIntervalEntry,
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

	// 时间同步配置区
	ntpServerContainer := container.NewBorder(
		nil,
		nil,
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), widget.NewLabel("NTP 服务器:")),
		nil,
		ntpServerEntry,
	)

	ntpIntervalContainer := container.NewBorder(
		nil,
		nil,
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), widget.NewLabel("检测间隔(秒):")),
		nil,
		ntpIntervalEntry,
	)

	ntpThresholdContainer := container.NewBorder(
		nil,
		nil,
		container.NewGridWrap(fyne.NewSize(float32(labelWidth), utils.LEBHeight), widget.NewLabel("偏差阈值(秒):")),
		nil,
		ntpThresholdEntry,
	)

	saveNTPBtn := widget.NewButton("保存配置", func() {
		if saveTimeSyncConfig() {
			timeSyncStatusLabel.SetText("配置已保存，下一轮检测生效")
		}
	})
	checkNTPBtn := widget.NewButton("立即检测", func() {
		if saveTimeSyncConfig() {
			checkTimeNow()
		}
	})

	timeSyncButtons := container.NewHBox(checkNTPBtn, saveNTPBtn)

	timeSyncBox := container.NewVBox(
		timeSyncStatusLabel, // 网络时间与偏差放在第一行
		ntpServerContainer,
		ntpIntervalContainer,
		ntpThresholdContainer,
		timeSyncButtons,
	)

	// 心跳配置卡片
	heartbeatConfigCard := container.NewVBox(
		widget.NewLabelWithStyle("心跳配置", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		heartbeatConfigBox,
	)

	// 系统信息卡片：输入框按 12 行内容高度显示，卡片不额外留白
	sysInfoCard := container.NewBorder(
		widget.NewLabelWithStyle("系统信息", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		sysInfoText,
	)

	// 时间同步卡片
	timeSyncCard := container.NewVBox(
		widget.NewLabelWithStyle("时间同步（仅检测，不修改系统时间）", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		timeSyncBox,
	)

	// 心跳日志卡片：输入框铺满卡片（卡片高度由布局决定，吃掉左列剩余高度）
	heartbeatLogCard := container.NewBorder(
		widget.NewLabelWithStyle("心跳日志", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		heartbeatLogText,
	)

	// 合并布局：左列两个文本框（上=系统信息、下=心跳日志），右列两个配置（上=心跳配置、下=时间同步）
	// 宽度固定为各占一半；同列卡片按内容高度从上往下紧密排列，多余高度只给最后一个卡片，避免出现大片空白
	logColumn := container.New(topAlignFillLayout{}, sysInfoCard, heartbeatLogCard)
	configColumn := container.New(topAlignFillLayout{}, heartbeatConfigCard, timeSyncCard)

	mainContent := container.NewGridWithColumns(2, logColumn, configColumn)

	// 左右两列铺满剩余空间
	monitorUI := container.NewBorder(
		topBar,      // top
		nil,         // bottom
		nil,         // left
		nil,         // right
		mainContent, // center
	)

	// 界面加载时自动刷新一次系统信息
	refreshSysInfo()

	return monitorUI
}

// topAlignFillLayout 垂直布局：前面所有对象按最小高度从上往下紧密排列，剩余高度全部给最后一个对象
// 用于同列多张卡片时，卡片不会被子对象撑开留下空白（系统信息、心跳日志等日志框尤其明显）
type topAlignFillLayout struct{}

func (l topAlignFillLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	min := fyne.NewSize(0, 0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		ms := o.MinSize()
		min.Width = fyne.Max(min.Width, ms.Width)
		min.Height += ms.Height
	}
	return min
}

func (l topAlignFillLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var pos float32
	last := -1
	for i, o := range objects {
		if o.Visible() {
			last = i
		}
	}
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		if i == last {
			h = fyne.Max(h, size.Height-pos) // 最后一个对象吃掉剩余高度
		}
		o.Move(fyne.NewPos(0, pos))
		o.Resize(fyne.NewSize(size.Width, h))
		pos += h
	}
}
