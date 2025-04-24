package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-uposs/utils"

	"github.com/minio/minio-go/v7"
)

// UploadImagesToMinio 上传本地路径中的所有图片到 minio
func UploadImagesToMinio(client *minio.Client, bucketName, localPath, minioPath string, api1URL, api2URL string, isScheduledTask bool, config *Config) (int, error) {
	// 检查存储桶是否存在
	exists, err := client.BucketExists(context.Background(), bucketName)
	if err != nil {
		return 0, fmt.Errorf("检查存储桶失败❌😅: %v", err)
	}
	if !exists {
		err = client.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return 0, fmt.Errorf("创建存储桶失败❌😅: %v", err)
		}
		logUploadMessage(fmt.Sprintf("存储桶 %s 已创建", bucketName), isScheduledTask)
	} else {
		logUploadMessage(fmt.Sprintf("存储桶 %s 已存在", bucketName), isScheduledTask)
	}

	fileCount := 0     // 统计处理文件数量
	uploadedCount := 0 // 统计成功上传的文件数量

	err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".jpeg") &&
			!strings.HasSuffix(strings.ToLower(info.Name()), ".jpg") &&
			!strings.HasSuffix(strings.ToLower(info.Name()), ".png") &&
			!strings.HasSuffix(strings.ToLower(info.Name()), ".gif") {
			return nil
		}

		fileCount++
		orderNumbers := utils.ParseImageName(info.Name())
		if len(orderNumbers) == 0 {
			logUploadMessage(fmt.Sprintf("无法从文件名解析编号: %s，删除此文件", info.Name()), isScheduledTask)
			err = os.Remove(path)
			if err != nil {
				logUploadMessage(fmt.Sprintf("删除无编号文件失败❌😅: %s, 错误: %v", path, err), isScheduledTask)
				return nil
			}
			logUploadMessage(fmt.Sprintf("已删除无编号文件: %s", path), isScheduledTask)
			return nil
		}

		logUploadMessage(fmt.Sprintf("从文件名 %s 解析到的编号: %s", info.Name(), strings.Join(orderNumbers, ", ")), isScheduledTask)

		validOrderFound := false
		var validOrderNumber string
		var explicitInvalid bool

		for _, orderNumber := range orderNumbers {
			for retry := 0; retry < 2; retry++ {
				logUploadMessage(fmt.Sprintf("正在向 API1 查询编号: %s (第%d次尝试)", orderNumber, retry+1), isScheduledTask)
				apiResponse, err := utils.QueryAPI1(api1URL, orderNumber)
				if err != nil {
					logUploadMessage(fmt.Sprintf("API1 查询失败❌😅: 编号: %s 第%d次尝试 错误: %v", orderNumber, retry+1, err), isScheduledTask)
					if retry < 1 {
						logUploadMessage("等待20秒后重试...", isScheduledTask)
						time.Sleep(20 * time.Second)
					}
					continue
				}
				if strings.HasPrefix(apiResponse, config.API1Response1) {
					logUploadMessage(fmt.Sprintf("API1 查询成功，编号: %s 有效, 响应: %s", orderNumber, apiResponse), isScheduledTask)
					validOrderFound = true
					validOrderNumber = orderNumber
					break
				}
				if strings.HasPrefix(apiResponse, config.API1Response2) {
					logUploadMessage(fmt.Sprintf("API1 查询返回无效状态: 编号: %s, 响应: %s", orderNumber, apiResponse), isScheduledTask)
					explicitInvalid = true
					break
				}
				logUploadMessage(fmt.Sprintf("跳过此文件处理，API1 返回未定义响应: 编号: %s, 响应: %s", orderNumber, apiResponse), isScheduledTask)
				return nil // 不中断整个流程，仅跳过当前文件
			}
			if validOrderFound || explicitInvalid {
				break
			}
		}

		if !validOrderFound && explicitInvalid {
			logUploadMessage(fmt.Sprintf("文件 %s 中没有有效编号（定义无效状态），删除此文件", info.Name()), isScheduledTask)
			err := os.Remove(path)
			if err != nil {
				logUploadMessage(fmt.Sprintf("删除无效编号文件失败❌😅: %s, 错误: %v", path, err), isScheduledTask)
			} else {
				logUploadMessage(fmt.Sprintf("已删除无效编号文件: %s", path), isScheduledTask)
			}
			return nil
		}

		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			logUploadMessage(fmt.Sprintf("获取相对路径失败❌😅: %v", err), isScheduledTask)
			return nil
		}
		var datePath string
		if filepath.Dir(relPath) == "." {
			datePath = time.Now().Format("2006.01.02")
		} else {
			datePath = filepath.Dir(relPath)
		}

		//构造 minio 文件路径
		minioFilePath := fmt.Sprintf("%s/%s/%s", minioPath, datePath, info.Name())
		minioFilePath = strings.ReplaceAll(minioFilePath, "\\", "/")

		//上传文件到 minio
		_, err = client.FPutObject(context.Background(), bucketName, minioFilePath, path, minio.PutObjectOptions{})
		if err != nil {
			logUploadMessage(fmt.Sprintf("上传文件失败❌😅: %s -> %s, 错误: %v", path, minioFilePath, err), isScheduledTask)
			return nil
		}

		fileUrl := fmt.Sprintf("%s/%s/%s", config.PublicUrl, bucketName, minioFilePath)
		logUploadMessage("文件上传成功，向 API2 推送编号文件访问地址", isScheduledTask)

		// 推送到API2
		var api2Err error
		for retry := 0; retry <= 1; retry++ {
			_, api2Err = utils.PushToAPI2(api2URL, validOrderNumber, fileUrl)
			if api2Err == nil {
				logUploadMessage(fmt.Sprintf("推送到 API2 成功😎 (第%d次尝试)，编号: %s，文件访问地址: %s", retry+1, validOrderNumber, fileUrl), isScheduledTask)
				err := os.Remove(path)
				if err == nil {
					logUploadMessage(fmt.Sprintf("本地文件已删除: %s", path), isScheduledTask)
				}
				uploadedCount++
				break
			}
			if retry == 0 {
				time.Sleep(20 * time.Second)
			}
		}
		if api2Err != nil {
			logUploadMessage("第 2 次推送 API2 失败❌😅，跳过此推送", isScheduledTask)
		}
		return nil
	})

	if fileCount > 0 {
		logUploadMessage(fmt.Sprintf("共处理 %d 个文件", fileCount), isScheduledTask)
	}
	return uploadedCount, err
}

// UploadImages 根据配置上传本地路径中的所有图片到 minio
func UploadImages(config *Config) error {
	return UploadImagesWithTaskType(config, true) // 默认为计划任务
}

// UploadImagesWithTaskType 根据配置上传本地路径中的所有图片到 minio，指定任务类型
func UploadImagesWithTaskType(config *Config, isScheduledTask bool) error {
	hasImages, err := checkForImages(config.LocalFolder)
	if err != nil {
		return fmt.Errorf("检查图片文件失败❌😅: %v", err)
	}
	if !hasImages {
		return fmt.Errorf("无文件可上传")
	}

	client, err := InitMinioClient(config, config.UseSSL)
	if err != nil {
		return fmt.Errorf("初始化 minio 客户端失败❌😅: %v", err)
	}
	if err := TestConnection(client); err != nil {
		return fmt.Errorf("minio 连接测试失败❌😅: %v", err)
	}

	machineCode := config.MachineCode
	if machineCode == "" {
		return fmt.Errorf("配置中的 machine_code 不能为空")
	}

	logUploadMessage(fmt.Sprintf("开始上传图片，本地路径: %s, minio 路径: %s", config.LocalFolder, machineCode), isScheduledTask)

	uploadedCount, err := UploadImagesToMinio(client, config.BucketName, config.LocalFolder, machineCode, config.API1, config.API2, isScheduledTask, config)
	if err != nil {
		return fmt.Errorf("上传图片失败❌😅: %v", err)
	}

	if uploadedCount == 0 {
		logUploadMessage("所有文件均被跳过或处理失败❌😅，未成功上传任何图片", isScheduledTask)
	} else {
		logUploadMessage(fmt.Sprintf("图片上传完成，共上传 %d 张", uploadedCount), isScheduledTask)
	}

	return nil
}

// checkForImages 检查指定路径下是否有图片文件
func checkForImages(path string) (bool, error) {
	hasImages := false

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 检查是否为图片文件
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" {
			hasImages = true
			return filepath.SkipAll // 找到一个图片就停止遍历
		}

		return nil
	})

	return hasImages, err
}

// logUploadMessage 记录上传相关日志，根据任务类型选择不同日志记录函数
func logUploadMessage(message string, isScheduledTask bool) {
	if isScheduledTask {
		SchedLogToFile(message)
	} else {
		AutoLogToFile(message)
	}
}
