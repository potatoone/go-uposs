package main

import (
	"io"
	"os"
	"time"
)

// parseFolderNameToTime 将文件夹名称解析为时间对象
func parseFolderNameToTime(folderName string) (time.Time, error) {
	return time.Parse("2006.01.02", folderName)
}

// isFolderInTimeRange 检查文件夹名称是否在时间范围内
func isFolderInTimeRange(folderName string, isScheduledTask bool, config *Config) bool {
	if folderName == "." {
		return false
	}
	folderTime, err := parseFolderNameToTime(folderName)
	if err != nil {
		return false
	}

	now := time.Now()
	if !isScheduledTask {
		// 自动任务：上传今天和昨天文件夹中的文件
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayEnd := todayStart.AddDate(0, 0, 1)
		yesterdayStart := todayStart.AddDate(0, 0, -1)
		return (!folderTime.Before(yesterdayStart) && folderTime.Before(todayEnd))
	} else {
		// 定时任务：基于配置的时间范围
		startDate, err := time.Parse("2006.01.02", config.StartTime)
		if err != nil {
			return false
		}
		endDate, err := time.Parse("2006.01.02", config.EndTime)
		if err != nil {
			return false
		}
		return !folderTime.Before(startDate) && !folderTime.After(endDate)
	}
}

// copySingleFile 将单个文件从源路径复制到目标路径
func copySingleFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}
