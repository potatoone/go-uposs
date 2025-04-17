package main

import (
	"fmt"
	"go-uposs/database"
	"go-uposs/utils" // 导入 utils 包
	"log"
	"os"
	"path/filepath"
	"time"
)

// CleanConfig 清理配置
type CleanConfig struct {
	StartTime string `json:"cleaStartTime"` // 开始时间
	EndTime   string `json:"cleanEndTime"`  // 结束时间
}

// LoadCleanConfig 加载清理配置
func LoadCleanConfig(configPath string) (CleanConfig, error) {
	var config CleanConfig

	// 如果配置文件存在，从中加载配置
	if _, err := os.Stat(configPath); err == nil {
		mainConfig, err := LoadConfig(configPath)
		if err != nil {
			return config, fmt.Errorf("加载配置文件失败: %v", err)
		}

		// 设置清理配置
		config.StartTime = mainConfig.CleanStartTime
		config.EndTime = mainConfig.CleanEndTime
	}

	return config, nil
}

// cleanLogFilesByDateRange 按日期范围清理日志文件
func cleanLogFilesByDateRange(config CleanConfig, dryRun bool) (int, int64, error) {
	log.Printf("[CLEAN] 开始清理日志文件，时间范围: %s 到 %s\n", config.StartTime, config.EndTime)

	// 解析开始和结束日期
	startTime, err := time.Parse("2006.01.02", config.StartTime)
	if err != nil {
		return 0, 0, fmt.Errorf("解析开始日期失败: %v", err)
	}

	endTime, err := time.Parse("2006.01.02", config.EndTime)
	if err != nil {
		return 0, 0, fmt.Errorf("解析结束日期失败: %v", err)
	}

	// 结束日期调整到当天结束
	endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, endTime.Location())

	var removedCount int
	var totalSize int64

	// 指定要清理的日志路径 - 使用预定义的路径常量
	logPaths := []string{
		utils.AutoLogPath,  // 使用预定义的 auto 日志路径
		utils.SchedLogPath, // 使用预定义的 sched 日志路径
	}

	for _, logPath := range logPaths {

		// 检查目录是否存在
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			log.Printf("[CLEAN] 目录不存在，跳过: %s\n", logPath)
			continue
		}

		// 遍历日志目录中的文件
		err := filepath.Walk(logPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Printf("[CLEAN] 访问路径失败: %s - %v\n", path, err)
				return nil
			}

			// 只处理文件
			if !info.IsDir() {
				// 获取文件修改时间
				fileTime := info.ModTime()

				// 检查文件时间是否在指定范围内
				if (fileTime.Equal(startTime) || fileTime.After(startTime)) &&
					(fileTime.Equal(endTime) || fileTime.Before(endTime)) {
					fileSize := info.Size()
					log.Printf("[CLEAN] 发现符合条件的日志文件: %s (大小: %d 字节)\n", path, fileSize)

					if !dryRun {
						if err := os.Remove(path); err != nil {
							log.Printf("[CLEAN] 删除文件失败: %s - %v\n", path, err)
							return nil
						}
					}
					removedCount++
					totalSize += fileSize
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("[CLEAN] 遍历目录失败: %s - %v\n", logPath, err)
		}
	}

	action := "已删除"
	if dryRun {
		action = "将删除"
	}
	log.Printf("[CLEAN] 日志清理完成: %s %d 个文件，总大小 %.2f MB\n",
		action, removedCount, float64(totalSize)/(1024*1024))

	return removedCount, totalSize, nil
}

// cleanDatabaseRecordsByDateRange 按日期范围清理数据库记录
func cleanDatabaseRecordsByDateRange(config CleanConfig, dryRun bool) (int64, error) {
	// 解析日期范围
	_, err := time.Parse("2006.01.02", config.StartTime)
	if err != nil {
		return 0, fmt.Errorf("解析开始日期失败: %v", err)
	}

	_, err = time.Parse("2006.01.02", config.EndTime)
	if err != nil {
		return 0, fmt.Errorf("解析结束日期失败: %v", err)
	}

	// 使用与文件夹名称相同的格式
	startDateStr := config.StartTime
	endDateStr := config.EndTime

	log.Printf("[CLEAN] 清理从 %s 到 %s 的数据库记录", startDateStr, endDateStr)

	var totalDeleted int64

	// 需要清理的表和它们的日期字段
	tablesAndFields := map[string]string{
		"auto_copy_records":      "copy_dir",
		"scheduled_copy_records": "copy_dir",
	}

	if !dryRun {
		// 实际执行删除操作
		for table, dateField := range tablesAndFields {
			query := fmt.Sprintf("DELETE FROM %s WHERE %s BETWEEN ? AND ?", table, dateField)

			log.Printf("[CLEAN] 执行SQL: %s 参数: %s, %s", query, startDateStr, endDateStr)

			result, err := database.ExecDB(query, startDateStr, endDateStr)
			if err != nil {
				log.Printf("[CLEAN] 清理%s记录失败: %v", table, err)
				continue
			}

			rowsDeleted, _ := result.RowsAffected()
			totalDeleted += rowsDeleted
			log.Printf("[CLEAN] 已从%s中删除 %d 条记录", table, rowsDeleted)
		}

		// 执行VACUUM优化数据库大小
		_, err := database.ExecDB("VACUUM")
		if err != nil {
			log.Printf("[CLEAN] 清理数据库失败: %v", err)
		} else {
			log.Println("[CLEAN] 数据库清理完成")
		}
	} else {
		// 干运行模式，只计数不删除
		for table, dateField := range tablesAndFields {
			query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s BETWEEN ? AND ?", table, dateField)

			log.Printf("[CLEAN] 执行SQL: %s 参数: %s, %s", query, startDateStr, endDateStr)

			var count int64
			row := database.QueryRowDB(query, startDateStr, endDateStr)
			if err := row.Scan(&count); err != nil {
				log.Printf("[CLEAN] 计算%s记录数失败: %v", table, err)
				continue
			}

			totalDeleted += count
			log.Printf("[CLEAN] 将从%s中删除 %d 条记录", table, count)
		}
	}

	return totalDeleted, nil
}

func init() {
	// 确保日志目录存在
	logDir := filepath.Dir(utils.SysLogPath) // 使用 SysLogPath 来获取目录
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Fatalf("创建日志目录失败: %v", err)
	}

	// 打开日志文件
	logFile, err := os.OpenFile(utils.SysLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("打开日志文件失败: %v", err)
	}

	// 设置日志输出到文件
	log.SetOutput(logFile)
}
