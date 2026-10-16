package main

import (
	"fmt"
	"go-uposs/internal/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

var (
	schedLogger *Logger // 计划任务日志记录器实例
	autoLogger  *Logger // 自动任务日志记录器实例
	maxLogLines = 20    // 最大保留日志行数

	// 日志写入互斥锁
	logMutex sync.Mutex

	// 任务日志写入互斥锁，避免并发处理文件时日志内容交叉
	taskLogMutex sync.Mutex
)

// Logger 用于处理日志的记录
type Logger struct {
	LogDir string
	Date   string
}

// NewLogger 创建新的 Logger 实例
func NewLogger(logDir string) *Logger {
	// 确保日志目录存在
	err := os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		fmt.Printf("创建日志目录失败: %v\n", err)
	}

	return &Logger{
		LogDir: logDir,
		Date:   time.Now().Format("2006.01.02"),
	}
}

// InitSysLogger 初始化系统日志
func InitSysLogger() error {
	// 确保日志目录存在
	logDir := filepath.Dir(utils.SysLogPath)
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 确保日志文件可写入
	file, err := os.OpenFile(utils.SysLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开系统日志文件: %v", err)
	}
	defer file.Close()

	return nil
}

// SysLogToFile 向系统日志文件写入日志
func SysLogToFile(message string) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	// 确保日志目录存在
	logDir := filepath.Dir(utils.SysLogPath)
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 打开或创建日志文件
	file, err := os.OpenFile(utils.SysLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开系统日志文件: %v", err)
	}
	defer file.Close()

	// 在消息末尾添加换行符（如果没有的话）
	if len(message) > 0 && message[len(message)-1] != '\n' {
		message += "\n"
	}

	// 格式化日志内容，包含时间戳
	logMessage := fmt.Sprintf("%s %s", time.Now().Format("2006-01-02 15:04:05"), message)

	// 写入日志
	if _, err := file.WriteString(logMessage); err != nil {
		return fmt.Errorf("写入日志失败: %v", err)
	}

	return nil
}

// ...
// ...
// ...
// 任务日志记录器部分
// InitSchedLogger 初始化计划任务日志记录器
func InitSchedLogger(logDir string) {
	schedLogger = NewLogger(logDir)
}

// InitAutoLogger 初始化自动任务日志记录器
func InitAutoLogger(logDir string) {
	autoLogger = NewLogger(logDir)
}

// 通用日志封装函数：记录日志到文件并更新 UI// logToUIAndFile 安全地将日志写入文件并更新 UI
// 通用日志封装函数：记录日志到文件并更新 UI
func logToUIAndFile(logger *Logger, logWidget *widget.Entry, message string, taskLogLines int) error {
	if logger == nil {
		return fmt.Errorf("日志记录器未初始化")
	}

	// 串行写入，避免并发任务导致日志内容交叉或文件写入冲突
	taskLogMutex.Lock()
	defer taskLogMutex.Unlock()

	// 获取当前日期，并检查是否需要更换日志文件
	currentDate := time.Now().Format("2006.01.02")
	if currentDate != logger.Date {
		logger.Date = currentDate
	}

	// 获取当前日志文件路径
	logFilePath := fmt.Sprintf("%s/%s.log", logger.LogDir, logger.Date)

	// 打开或创建日志文件
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开日志文件: %v", err)
	}
	defer file.Close()

	// 格式化日志内容，包含时间戳
	logMessage := fmt.Sprintf("%s %s", time.Now().Format("2006-01-02 15:04:05"), message)

	// 写入日志文件
	if _, err := file.WriteString(logMessage + "\n"); err != nil {
		return fmt.Errorf("写入日志失败: %v", err)
	}

	// UI 更新走批量刷新：
	// 批量任务时每秒可能产生上百条日志，逐条 SetText 会让主线程忙于重排文本、拖慢界面。
	enqueueUILog(logWidget, message, taskLogLines)

	return nil
}

// ==================== UI 日志批量刷新 ====================
//
// 批量任务（复制/压缩/上传）每秒可能写入上百条日志，
// 若每条都调用一次 fyne.Do + SetText，主线程会被大量文本重排与重绘占满，
// 表现为界面卡顿、切换标签页无响应。
// 这里把日志先入队，由后台协程按固定间隔合并刷新一次。

const uiLogFlushInterval = 300 * time.Millisecond

// uiLogBuffer 单个日志框的待刷新队列
type uiLogBuffer struct {
	mu    sync.Mutex
	lines []string
	limit int
	flush bool // 是否有待刷新内容
}

var (
	uiLogBuffersMu sync.Mutex
	uiLogBuffers   = make(map[*widget.Entry]*uiLogBuffer)
	uiLogFlusher   sync.Once
)

// enqueueUILog 将日志加入刷新队列
func enqueueUILog(widget *widget.Entry, message string, limit int) {
	if widget == nil {
		return
	}

	uiLogBuffersMu.Lock()
	buf, ok := uiLogBuffers[widget]
	if !ok {
		buf = &uiLogBuffer{}
		uiLogBuffers[widget] = buf
	}
	uiLogBuffersMu.Unlock()

	buf.mu.Lock()
	buf.lines = append(buf.lines, message)
	// 队列只需保留最近 limit 行，避免长时间任务堆积过多文本
	if len(buf.lines) > limit {
		buf.lines = buf.lines[len(buf.lines)-limit:]
	}
	buf.limit = limit
	buf.flush = true
	buf.mu.Unlock()

	// 首次使用时启动统一的刷新协程
	uiLogFlusher.Do(func() {
		go func() {
			ticker := time.NewTicker(uiLogFlushInterval)
			defer ticker.Stop()
			for range ticker.C {
				flushUILogs()
			}
		}()
	})
}

// flushUILogs 把各日志框队列中的内容一次性合并写回控件
func flushUILogs() {
	uiLogBuffersMu.Lock()
	widgets := make([]*widget.Entry, 0, len(uiLogBuffers))
	for w := range uiLogBuffers {
		widgets = append(widgets, w)
	}
	uiLogBuffersMu.Unlock()

	for _, w := range widgets {
		uiLogBuffersMu.Lock()
		buf := uiLogBuffers[w]
		uiLogBuffersMu.Unlock()
		if buf == nil {
			continue
		}

		buf.mu.Lock()
		if !buf.flush || len(buf.lines) == 0 {
			buf.mu.Unlock()
			continue
		}
		pending := buf.lines
		buf.lines = nil
		buf.flush = false
		limit := buf.limit
		buf.mu.Unlock()

		// 读取控件当前文本并写回都必须发生在主线程，避免跨线程访问控件状态
		uiWidget, uiPending, uiLimit := w, strings.Join(pending, "\n"), limit
		fyne.Do(func() {
			currentText := uiWidget.Text
			if currentText != "" {
				currentText += "\n"
			}

			lines := strings.Split(currentText+uiPending, "\n")
			if len(lines) > uiLimit {
				lines = lines[len(lines)-uiLimit:]
			}
			uiWidget.SetText(strings.Join(lines, "\n"))
		})
	}
}

// AutoLogToUIAndFile 向自动任务日志和 UI 日志写入消息
func AutoLogToFile(message string) error {
	return logToUIAndFile(autoLogger, autoLogText, message, 20)
}

// SchedLogToUIAndFile 向计划任务日志和 UI 日志写入消息
func SchedLogToFile(message string) error {
	return logToUIAndFile(schedLogger, schedLogText, message, 17)
}

// ...
// ...
// ...
// 封装日志更新函数
// updateLog 带标签地更新日志到指定的 Entry 控件，并确保线程安全
// 参数：
//
//	logWidget - 日志输出目标的 Entry 控件
//	tag       - 日志来源标签，例如 "[OSS配置]"
//	message   - 实际日志内容（不需要时间戳，函数内部自动添加）
func updateLog(logWidget *widget.Entry, tag, message string) {
	fyne.Do(func() {
		// 构建带时间戳和标签的日志消息
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fullMessage := fmt.Sprintf("%s %s %s", timestamp, tag, message)

		// 取当前 Entry 的内容
		currentText := logWidget.Text

		// 将新消息插入到最前面（新日志在上）
		if currentText != "" {
			currentText = fullMessage + "\n" + currentText
		} else {
			currentText = fullMessage
		}

		// 限制最大行数（保留最新的 maxLogLines 行）
		lines := strings.Split(currentText, "\n")
		if len(lines) > maxLogLines {
			lines = lines[:maxLogLines]
		}

		// 设置更新后的日志内容
		logWidget.SetText(strings.Join(lines, "\n"))

		// 同步写入系统日志文件
		SysLogToFile(fmt.Sprintf("%s %s", tag, message))
	})
}
