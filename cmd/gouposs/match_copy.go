package main

import (
	"fmt"
	"go-uposs/internal/utils"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// MatchDatePattern 匹配日期格式的文件夹 (例如：2025.03.05)
func MatchDatePattern(folderName string) bool {
	re := regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`)
	return re.MatchString(folderName)
}

// fileConcurrencyLimiter 限制同时处理的文件数量
type fileConcurrencyLimiter struct {
	slots chan struct{}
}

// newFileConcurrencyLimiter 创建并发限制器，并发数小于 1 时按 1 处理
func newFileConcurrencyLimiter(concurrency int) *fileConcurrencyLimiter {
	if concurrency < 1 {
		concurrency = 1
	}
	return &fileConcurrencyLimiter{slots: make(chan struct{}, concurrency)}
}

// acquire 占用一个并发名额
func (l *fileConcurrencyLimiter) acquire() {
	l.slots <- struct{}{}
}

// release 释放一个并发名额
func (l *fileConcurrencyLimiter) release() {
	<-l.slots
}

// nameLockEntry 同名文件的互斥锁及其引用计数
type nameLockEntry struct {
	mu   sync.Mutex
	refs int
}

// nameLocker 保证同名文件不会被并发处理，保持串行时“已复制则跳过”的语义
type nameLocker struct {
	mu      sync.Mutex
	entries map[string]*nameLockEntry
}

func newNameLocker() *nameLocker {
	return &nameLocker{entries: make(map[string]*nameLockEntry)}
}

// lock 获取指定名称的锁，同名调用会串行执行
func (n *nameLocker) lock(key string) *nameLockEntry {
	n.mu.Lock()
	entry, ok := n.entries[key]
	if !ok {
		entry = &nameLockEntry{}
		n.entries[key] = entry
	}
	entry.refs++
	n.mu.Unlock()

	entry.mu.Lock()
	return entry
}

// unlock 释放指定名称的锁，无引用时清理记录
func (n *nameLocker) unlock(key string, entry *nameLockEntry) {
	entry.mu.Unlock()

	n.mu.Lock()
	entry.refs--
	if entry.refs == 0 {
		delete(n.entries, key)
	}
	n.mu.Unlock()
}

// copyContext 一次复制任务共用的上下文，保证并发限制在整次任务内生效
type copyContext struct {
	bufferSize   int
	limiter      *fileConcurrencyLimiter
	nameLocks    *nameLocker
	isAutoTask   bool
	dateRange    string
	orderNumbers string
}

// newCopyContext 根据配置创建复制任务上下文
func newCopyContext(config *Config, isAutoTask bool, dateRange string, orderNumbers string) *copyContext {
	return &copyContext{
		bufferSize:   config.IOBuffer,
		limiter:      newFileConcurrencyLimiter(config.CopyConcurrency),
		nameLocks:    newNameLocker(),
		isAutoTask:   isAutoTask,
		dateRange:    dateRange,
		orderNumbers: orderNumbers,
	}
}

// log 按任务类型写入对应日志文件
func (ctx *copyContext) log(message string) {
	if ctx.isAutoTask {
		AutoLogToFile(message)
	} else {
		SchedLogToFile(message)
	}
}

// CopyDir 复制目录及其内容
func CopyDir(src string, dst string, copyCtx *copyContext) error {
	return copyCtx.copyDir(src, dst)
}

// copyDir 复制目录及其内容，文件按配置的并发数量上限并行复制
func (ctx *copyContext) copyDir(src string, dst string) error {
	if ctx.bufferSize <= 0 {
		return fmt.Errorf("无效的缓冲区大小")
	}

	err := os.MkdirAll(dst, os.ModePerm)
	if err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %v", err)
	}

	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	// 只记录第一个错误，但继续等待已启动的复制任务结束
	recordErr := func(err error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
	}

	// 检查日期范围
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// 检查是否是目录
		if entry.IsDir() {
			if err := ctx.copyDir(srcPath, dstPath); err != nil {
				recordErr(err)
			}
			continue
		}

		// 非自动任务，检查编号
		if !ctx.isAutoTask && ctx.orderNumbers != "" {
			matchedFiles := matchFilesByNumbers(ctx.orderNumbers, []string{entry.Name()})
			if len(matchedFiles) == 0 {
				continue
			}
		}

		// 只检查数据库记录，不需要检查物理文件
		wg.Add(1)
		ctx.limiter.acquire() // 占用一个文件并发名额
		go func(srcPath, dstPath string) {
			defer wg.Done()
			defer ctx.limiter.release()

			if err := ctx.copyFile(srcPath, dstPath); err != nil {
				recordErr(err)
			}
		}(srcPath, dstPath)
	}

	wg.Wait()

	// 存在复制失败的文件时不记录成功日志
	if firstErr != nil {
		return firstErr
	}

	// 记录系统日志
	ctx.log(fmt.Sprintf("匹配到的源文件夹: %s, 目录复制成功: %s -> %s", src, src, dst))

	return nil
}

// copyFile 复制文件
func (ctx *copyContext) copyFile(src, dst string) error {
	fileName := filepath.Base(src)

	// 同名文件串行处理，避免并发时重复复制
	entry := ctx.nameLocks.lock(fileName)
	defer ctx.nameLocks.unlock(fileName, entry)

	// 检查文件是否已经复制过
	exists, err := utils.CheckFileExists(fileName, ctx.isAutoTask)
	if err != nil {
		// 数据库错误，记录到文件日志
		ctx.log(fmt.Sprintf("检查文件是否存在时出错: %v", err))
	} else if exists {
		// 普通操作信息，只记录到文件日志
		// logMsg := fmt.Sprintf("文件 %s 已经复制过，跳过", fileName)
		// if isAutoTask {
		// 	AutoLogToFile(logMsg)
		// } else {
		// 	SchedLogToFile(logMsg)
		// }
		return nil
	}

	srcFile, err := os.Open(src)
	if err != nil {
		// 错误信息，记录到文件
		ctx.log(fmt.Sprintf("无法打开源文件 %s: %v", src, err))
		return fmt.Errorf("无法打开源文件: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		// 记录错误到文件日志
		ctx.log(fmt.Sprintf("无法创建目标文件 %s: %v", dst, err))
		return fmt.Errorf("无法创建目标文件: %v", err)
	}
	defer dstFile.Close()

	buf := make([]byte, ctx.bufferSize)
	for {
		n, err := srcFile.Read(buf)
		if err != nil && err != io.EOF {
			// 记录错误到文件日志
			ctx.log(fmt.Sprintf("读取源文件 %s 失败: %v", src, err))
			return fmt.Errorf("读取源文件失败: %v", err)
		}
		if n == 0 {
			break
		}

		if _, err := dstFile.Write(buf[:n]); err != nil {
			// 记录错误到文件日志
			ctx.log(fmt.Sprintf("写入目标文件 %s 失败: %v", dst, err))
			return fmt.Errorf("写入目标文件失败: %v", err)
		}
	}

	// 获取复制源完整路径的最后路径 (即包含日期的文件夹)
	copyDir := filepath.Base(filepath.Dir(src))

	// 将复制记录添加到数据库，移除 parsedNames 参数
	if err = utils.RecordFileCopy(fileName, copyDir, ctx.isAutoTask); err != nil {
		// 数据库错误仅记录，不影响复制结果
		ctx.log(fmt.Sprintf("记录文件复制失败: %v", err))
	}

	// 成功复制的文件信息，记录到日志文件
	ctx.log(fmt.Sprintf("成功复制文件: %s %s -> %s %s", "源路径", fileName, "目的路径", fileName))

	return nil
}

// ScanAndCopyFolders 扫描并复制匹配选定日期范围内的文件夹
func ScanAndCopyFolders(config *Config, orderNumbers string) error {
	startDate, err := time.Parse("2006.01.02", config.StartTime)
	if err != nil {
		return fmt.Errorf("解析开始日期失败: %v", err)
	}

	endDate, err := time.Parse("2006.01.02", config.EndTime)
	if err != nil {
		return fmt.Errorf("解析结束日期失败: %v", err)
	}

	// 构建日期范围字符串，用于数据库记录
	dateRange := fmt.Sprintf("%s-%s", config.StartTime, config.EndTime)

	// 创建复制上下文，并发数量限制在整次任务内生效
	copyCtx := newCopyContext(config, false, dateRange, orderNumbers)

	return filepath.Walk(config.RemoteFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && MatchDatePattern(info.Name()) {
			folderDate, err := time.Parse("2006.01.02", info.Name())
			if err != nil {
				return err
			}

			if !folderDate.Before(startDate) && !folderDate.After(endDate) {
				dstPath := filepath.Join(config.LocalFolder, info.Name())
				err = CopyDir(path, dstPath, copyCtx)
				if err != nil {
					return fmt.Errorf("复制文件夹 %s 失败: %v", path, err)
				}
			}
		}

		return nil
	})
}

// ScanAndCopyFoldersForToday 扫描并复制匹配当前日期的文件夹
func ScanAndCopyFoldersForToday(config *Config) error {
	// 获取昨天和今天的日期字符串
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006.01.02")
	today := time.Now().Format("2006.01.02")

	// 自动任务日期范围：昨天和今天
	dateSet := map[string]bool{
		yesterday: true,
		today:     true,
	}

	// 自动任务没有特定的人工选择日期范围
	dateRange := ""

	// 创建复制上下文，并发数量限制在整次任务内生效
	copyCtx := newCopyContext(config, true, dateRange, "")

	return filepath.Walk(config.RemoteFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && dateSet[info.Name()] {
			dstPath := filepath.Join(config.LocalFolder, info.Name())
			err = CopyDir(path, dstPath, copyCtx) // 自动任务不需要编号，参数置为空字符串
			if err != nil {
				return fmt.Errorf("复制文件夹 %s 失败: %v", path, err)
			}
		}

		return nil
	})
}
