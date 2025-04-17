package main

import (
	"fmt"
	"go-uposs/utils"
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

	// 写入日志并换行
	if _, err := file.WriteString(logMessage + "\n"); err != nil {
		return fmt.Errorf("写入日志失败: %v", err)
	}

	// 在主线程上更新 UI
	fyne.Do(func() {
		// 获取当前日志文本框的内容
		currentText := logWidget.Text

		// 如果当前文本框内容非空，添加换行符
		if currentText != "" {
			currentText += "\n"
		}

		// 将新日志消息添加到现有内容中
		lines := strings.Split(currentText+message, "\n")
		// 限制日志文本框显示行数 taskLogLines
		if len(lines) > taskLogLines {
			lines = lines[len(lines)-taskLogLines:]
		}

		// 更新 UI 组件的文本
		logWidget.SetText(strings.Join(lines, "\n"))
	})

	return nil
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
