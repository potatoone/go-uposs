package main

import (
	"fmt"
	"go-uposs/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	logMessage := fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05"), message)

	// 写入日志
	if _, err := file.WriteString(logMessage); err != nil {
		return fmt.Errorf("写入日志失败: %v", err)
	}

	return nil
}

// 任务日志记录器部分
// InitSchedLogger 初始化计划任务日志记录器
func InitSchedLogger(logDir string) {
	schedLogger = NewLogger(logDir)
}

// InitAutoLogger 初始化自动任务日志记录器
func InitAutoLogger(logDir string) {
	autoLogger = NewLogger(logDir)
}

// AutoLogToFile 向自动任务日志文件写入日志
func AutoLogToFile(message string) error {
	if autoLogger == nil {
		return fmt.Errorf("自动任务日志记录器未初始化")
	}
	err := autoLogger.LogToFile(message)
	if err != nil {
		return err
	}

	// 更新UI日志，仅保留最新的maxLogLines行
	currentLines := strings.Split(autoLogText.Text, "\n")
	if len(currentLines) > maxLogLines {
		// 只保留最新的maxLogLines行
		newLines := currentLines[len(currentLines)-maxLogLines:]
		autoLogText.SetText(strings.Join(newLines, "\n"))
	}

	// 添加新日志到末尾
	if autoLogText.Text != "" {
		autoLogText.SetText(autoLogText.Text + "\n" + message)
	} else {
		autoLogText.SetText(message)
	}

	return nil
}

// SchedLogToFile 向计划任务日志文件写入日志
func SchedLogToFile(message string) error {
	if schedLogger == nil {
		return fmt.Errorf("计划任务日志记录器未初始化")
	}
	err := schedLogger.LogToFile(message)
	if err != nil {
		return err
	}

	// 更新UI日志，仅保留最新的maxLogLines行
	currentLines := strings.Split(schedLogText.Text, "\n")
	if len(currentLines) > maxLogLines {
		// 只保留最新的maxLogLines行
		newLines := currentLines[len(currentLines)-maxLogLines:]
		schedLogText.SetText(strings.Join(newLines, "\n"))
	}

	// 添加新日志到末尾
	if schedLogText.Text != "" {
		schedLogText.SetText(schedLogText.Text + "\n" + message)
	} else {
		schedLogText.SetText(message)
	}

	return nil
}

// LogToFile 记录日志到文件的全局函数 (已废弃，保留用于兼容)
func LogToFile(message string) error {
	// 优先使用自动任务日志记录器
	if autoLogger != nil {
		return autoLogger.LogToFile(message)
	}
	// 否则尝试使用计划任务日志记录器
	if schedLogger != nil {
		return schedLogger.LogToFile(message)
	}
	return fmt.Errorf("日志记录器未初始化")
}

// getLogFilePath 获取当前日志文件路径
func (l *Logger) getLogFilePath() string {
	return fmt.Sprintf("%s/%s.log", l.LogDir, l.Date)
}

// LogToFile 追加日志到文件
func (l *Logger) LogToFile(message string) error {
	currentDate := time.Now().Format("2006.01.02")
	if currentDate != l.Date {
		l.Date = currentDate
	}

	logFilePath := l.getLogFilePath()

	// 打开或创建日志文件
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开日志文件: %v", err)
	}
	defer file.Close()

	// 格式化日志内容，包含时间戳
	logMessage := fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05"), message)

	// 写入日志并换行
	if _, err := file.WriteString(logMessage + "\n"); err != nil { // 确保换行
		return fmt.Errorf("写入日志失败: %v", err)
	}

	return nil
}
