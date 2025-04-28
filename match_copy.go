package main

import (
	"fmt"
	"go-uposs/utils"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// MatchDatePattern 匹配日期格式的文件夹 (例如：2025.03.05)
func MatchDatePattern(folderName string) bool {
	re := regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`)
	return re.MatchString(folderName)
}

// CopyDir 复制目录及其内容
func CopyDir(src string, dst string, bufferSize int, isAutoTask bool, dateRange string, orderNumbers string) error {
	if bufferSize <= 0 {
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

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = CopyDir(srcPath, dstPath, bufferSize, isAutoTask, dateRange, orderNumbers)
			if err != nil {
				return err
			}
		} else {
			if !isAutoTask && orderNumbers != "" {
				matchedFiles := matchFilesByNumbers(orderNumbers, []string{entry.Name()})
				if len(matchedFiles) == 0 {
					continue
				}
			}
			// 现在只检查数据库记录，不需要检查物理文件
			err = CopyFile(srcPath, dstPath, bufferSize, isAutoTask, dateRange)
			if err != nil {
				return err
			}
		}
	}

	// 记录系统日志
	logMessage := fmt.Sprintf("匹配到的源文件夹: %s, 目录复制成功: %s -> %s", src, src, dst)
	if isAutoTask {
		AutoLogToFile(logMessage)
	} else {
		SchedLogToFile(logMessage)
	}

	return nil
}

// CopyFile 复制文件
func CopyFile(src, dst string, bufferSize int, isAutoTask bool, dateRange string) error {
	fileName := filepath.Base(src)

	// 检查文件是否已经复制过
	exists, err := utils.CheckFileExists(fileName, isAutoTask)
	if err != nil {
		// 数据库错误，记录到文件日志
		logMsg := fmt.Sprintf("检查文件是否存在时出错: %v", err)
		if isAutoTask {
			AutoLogToFile(logMsg)
		} else {
			SchedLogToFile(logMsg)
		}
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
		logMsg := fmt.Sprintf("无法打开源文件 %s: %v", src, err)
		if isAutoTask {
			AutoLogToFile(logMsg)
		} else {
			SchedLogToFile(logMsg)
		}
		// 删除对已移除函数的调用
		return fmt.Errorf("无法打开源文件: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		// 记录错误到文件日志
		logMsg := fmt.Sprintf("无法创建目标文件 %s: %v", dst, err)
		if isAutoTask {
			AutoLogToFile(logMsg)
		} else {
			SchedLogToFile(logMsg)
		}
		return fmt.Errorf("无法创建目标文件: %v", err)
	}
	defer dstFile.Close()

	buf := make([]byte, bufferSize)
	for {
		n, err := srcFile.Read(buf)
		if err != nil && err != io.EOF {
			// 记录错误到文件日志
			logMsg := fmt.Sprintf("读取源文件 %s 失败: %v", src, err)
			if isAutoTask {
				AutoLogToFile(logMsg)
			} else {
				SchedLogToFile(logMsg)
			}
			return fmt.Errorf("读取源文件失败: %v", err)
		}
		if n == 0 {
			break
		}

		if _, err := dstFile.Write(buf[:n]); err != nil {
			// 记录错误到文件日志
			logMsg := fmt.Sprintf("写入目标文件 %s 失败: %v", dst, err)
			if isAutoTask {
				AutoLogToFile(logMsg)
			} else {
				SchedLogToFile(logMsg)
			}
			return fmt.Errorf("写入目标文件失败: %v", err)
		}
	}

	// 获取复制源完整路径的最后路径 (即包含日期的文件夹)
	copyDir := filepath.Base(filepath.Dir(src))

	// 将复制记录添加到数据库，移除 parsedNames 参数
	if err = utils.RecordFileCopy(fileName, copyDir, isAutoTask); err != nil {
		// 数据库错误仅记录，不影响复制结果
		logMsg := fmt.Sprintf("记录文件复制失败: %v", err)
		if isAutoTask {
			AutoLogToFile(logMsg)
		} else {
			SchedLogToFile(logMsg)
		}
	}

	// 成功复制的文件信息，记录到日志文件
	logMsg := fmt.Sprintf("成功复制文件: %s %s -> %s %s", "源路径", fileName, "目的路径", fileName)
	if isAutoTask {
		AutoLogToFile(logMsg)
	} else {
		SchedLogToFile(logMsg)
	}

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
				err = CopyDir(path, dstPath, config.IOBuffer, false, dateRange, orderNumbers)
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

	return filepath.Walk(config.RemoteFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && dateSet[info.Name()] {
			dstPath := filepath.Join(config.LocalFolder, info.Name())
			err = CopyDir(path, dstPath, config.IOBuffer, true, dateRange, "") // 自动任务不需要编号，参数置为空字符串
			if err != nil {
				return fmt.Errorf("复制文件夹 %s 失败: %v", path, err)
			}
		}

		return nil
	})
}
