package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-uposs/utils"

	"github.com/minio/minio-go/v7"
)

// UploadImagesToMinio 上传本地路径中的所有图片到 MinIO
func UploadImagesToMinio(client *minio.Client, bucketName, localPath, minioPath string, api1URL, api2URL string, isScheduledTask bool) error {
	// 检查存储桶是否存在，若不存在则创建
	exists, err := client.BucketExists(context.Background(), bucketName)
	if err != nil {
		return fmt.Errorf("检查存储桶失败: %v", err)
	}
	if !exists {
		err = client.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("创建存储桶失败: %v", err)
		}
		logUploadMessage(fmt.Sprintf("存储桶 %s 已创建", bucketName), isScheduledTask)
	} else {
		logUploadMessage(fmt.Sprintf("存储桶 %s 已存在", bucketName), isScheduledTask)
	}

	// 文件计数器
	fileCount := 0

	// 遍历本地路径中的所有文件
	err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 如果是目录，跳过
		if info.IsDir() {
			return nil
		}

		// 只处理图片文件
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".jpeg") &&
			!strings.HasSuffix(strings.ToLower(info.Name()), ".jpg") &&
			!strings.HasSuffix(strings.ToLower(info.Name()), ".png") &&
			!strings.HasSuffix(strings.ToLower(info.Name()), ".gif") {
			return nil
		}

		// 增加文件计数
		fileCount++

		// 解析文件名获取单号,utils包内名称切割工具divide
		orderNumbers := utils.ParseImageName(info.Name())
		if len(orderNumbers) == 0 {
			logUploadMessage(fmt.Sprintf("无法从文件名解析单号: %s，删除此文件", info.Name()), isScheduledTask)
			err = os.Remove(path)
			if err != nil {
				logUploadMessage(fmt.Sprintf("删除无单号文件失败: %s, 错误: %v", path, err), isScheduledTask)
				return fmt.Errorf("删除无单号文件失败: %v", err)
			}
			logUploadMessage(fmt.Sprintf("已删除无单号文件: %s", path), isScheduledTask)
			return nil
		}

		// 查询API1确认是否有有效单号
		validOrderFound := false
		maxRetries := 2 // 最大重试次数

		// 记录提取到的所有单号
		logUploadMessage(fmt.Sprintf("从文件名 %s 解析到的单号: %s", info.Name(), strings.Join(orderNumbers, ", ")), isScheduledTask)

		// 遍历所有解析出的单号，查询API1
		for _, orderNumber := range orderNumbers {
			for retry := 0; retry < maxRetries; retry++ {
				// 调用API1进行查询
				logUploadMessage(fmt.Sprintf("正在查询单号: %s (第%d次尝试)", orderNumber, retry+1), isScheduledTask)
				apiResponse, err := QueryAPI1(api1URL, orderNumber)
				if err != nil {
					logUploadMessage(fmt.Sprintf("API1查询失败(第%d次尝试): 单号: %s, 错误: %v", retry+1, orderNumber, err), isScheduledTask)
					if retry < maxRetries-1 {
						logUploadMessage("等待20秒后重试...", isScheduledTask)
						time.Sleep(20 * time.Second)
						continue
					}
				} else if strings.HasPrefix(apiResponse, "200:") {
					logUploadMessage(fmt.Sprintf("API1查询成功: 单号: %s 有效, 响应: %s", orderNumber, apiResponse), isScheduledTask)
					validOrderFound = true
					break
				} else {
					logUploadMessage(fmt.Sprintf("API1查询返回非200状态: 单号: %s, 响应: %s", orderNumber, apiResponse), isScheduledTask)
					break // 此单号无效，不再重试，直接检查下一个单号
				}
			}

			if validOrderFound {
				break // 只要找到一个有效单号就可以停止查询
			}
		}

		// 如果没有有效单号，删除此文件并跳过上传
		if !validOrderFound {
			logUploadMessage(fmt.Sprintf("文件 %s 中没有有效单号，删除此文件", info.Name()), isScheduledTask)
			err = os.Remove(path)
			if err != nil {
				logUploadMessage(fmt.Sprintf("删除无效单号文件失败: %s, 错误: %v", path, err), isScheduledTask)
				return fmt.Errorf("删除无效单号文件失败: %v", err)
			}
			logUploadMessage(fmt.Sprintf("已删除无效单号文件: %s", path), isScheduledTask)
			return nil
		}

		// 提取文件所在的文件夹名称作为日期路径
		// 获取文件的相对路径
		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			logUploadMessage(fmt.Sprintf("获取相对路径失败: %v", err), isScheduledTask)
			return err
		}

		// 提取文件所在的文件夹名称
		// 如果文件直接在本地文件夹中，则使用当前日期
		var datePath string
		if filepath.Dir(relPath) == "." {
			// 文件在根目录中，使用当前日期
			datePath = time.Now().Format("2006.01.02")
			logUploadMessage(fmt.Sprintf("文件 %s 直接位于根目录，使用当前日期: %s", info.Name(), datePath), isScheduledTask)
		} else {
			// 提取文件所在目录名作为日期路径
			datePath = filepath.Dir(relPath)
			logUploadMessage(fmt.Sprintf("文件 %s 位于子目录中，使用目录名作为日期路径: %s", info.Name(), datePath), isScheduledTask)
		}

		// 构建路径: 机器码/日期/原文件名
		// MinIO 路径应该始终使用正斜杠，无论客户端操作系统是什么
		minioFilePath := fmt.Sprintf("%s/%s/%s", minioPath, datePath, info.Name())
		// 确保路径中的斜杠是正斜杠
		minioFilePath = strings.ReplaceAll(minioFilePath, "\\", "/")

		logUploadMessage(fmt.Sprintf("构建对象路径: %s", minioFilePath), isScheduledTask)

		// 文件有有效单号，上传到 MinIO
		logUploadMessage(fmt.Sprintf("开始上传文件: %s -> %s", path, minioFilePath), isScheduledTask)
		_, err = client.FPutObject(context.Background(), bucketName, minioFilePath, path, minio.PutObjectOptions{})
		if err != nil {
			logUploadMessage(fmt.Sprintf("上传文件失败: %s -> %s, 错误: %v", path, minioFilePath, err), isScheduledTask)
			return fmt.Errorf("上传文件失败: %v", err)
		}

		// 获取文件的公开访问地址
		fileURL := fmt.Sprintf("%s/%s/%s", client.EndpointURL(), bucketName, minioFilePath)
		logUploadMessage(fmt.Sprintf("文件上传成功: %s -> %s", path, minioFilePath), isScheduledTask)
		logUploadMessage(fmt.Sprintf("文件访问地址: %s", fileURL), isScheduledTask)

		// 将所有解析出的单号和文件URL推送到API2
		logUploadMessage(fmt.Sprintf("推送文件到API2: 单号: %s, 地址: %s", strings.Join(orderNumbers, ", "), fileURL), isScheduledTask)

		// 添加重试逻辑
		maxApi2Retries := 1 // 最大重试次数为1
		var api2Response string
		var api2Err error

		for retry := 0; retry <= maxApi2Retries; retry++ {
			logUploadMessage(fmt.Sprintf("推送到API2 (第%d次尝试): 单号: %s", retry+1, strings.Join(orderNumbers, ", ")), isScheduledTask)
			api2Response, api2Err = PushToAPI2(api2URL, orderNumbers, fileURL)

			if api2Err != nil {
				logUploadMessage(fmt.Sprintf("推送到API2失败(第%d次尝试): 单号: %s, 错误: %v",
					retry+1, strings.Join(orderNumbers, ", "), api2Err), isScheduledTask)
				if retry < maxApi2Retries {
					logUploadMessage("等待20秒后重试推送到API2...", isScheduledTask)
					time.Sleep(20 * time.Second)
					continue
				}
			} else {
				logUploadMessage(fmt.Sprintf("推送到API2成功: 单号: %s, 响应: %s",
					strings.Join(orderNumbers, ", "), api2Response), isScheduledTask)
				break
			}
		}

		if api2Err != nil {
			logUploadMessage(fmt.Sprintf("推送到API2最终失败: 单号: %s, 所有重试均失败",
				strings.Join(orderNumbers, ", ")), isScheduledTask)
			// 注意：我们不返回错误，允许继续处理并删除文件，因为上传到MinIO已成功
		}

		// 删除本地文件
		logUploadMessage(fmt.Sprintf("正在删除本地文件: %s", path), isScheduledTask)
		err = os.Remove(path)
		if err != nil {
			logUploadMessage(fmt.Sprintf("删除本地文件失败: %s, 错误: %v", path, err), isScheduledTask)
			return fmt.Errorf("删除本地文件失败: %v", err)
		}
		logUploadMessage(fmt.Sprintf("本地文件已删除: %s", path), isScheduledTask)

		return nil
	})

	// 文件计数统计
	if fileCount > 0 {
		logUploadMessage(fmt.Sprintf("共处理 %d 个文件", fileCount), isScheduledTask)
	} else {
		// 这个情况应该不会发生，因为我们已经提前检查了是否有文件，但为了健壮性还是加上
		logUploadMessage("无文件被处理", isScheduledTask)
	}

	return err
}

// QueryAPI1 调用API1进行单号验证
func QueryAPI1(apiURL, orderNumber string) (string, error) {
	// 方法2：使用表单数据
	formData := url.Values{}
	formData.Set("orderCode", orderNumber)

	// 创建POST请求
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头为表单数据
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// 设置超时时间
	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	// 发送POST请求
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析JSON响应
	var response struct {
		Code            int    `json:"code"`
		Order           string `json:"order"`
		SystemOrderCode string `json:"systemOrderCode"`
		Number          int    `json:"number"`
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", fmt.Errorf("解析JSON响应失败: %v, 响应内容: %s", err, string(body))
	}

	// 检查响应状态码
	if response.Code == 200 {
		return fmt.Sprintf("200:%s", response.Order), nil
	}

	return fmt.Sprintf("%d:%s", response.Code, string(body)), nil
}

// PushToAPI2 推送单号和文件URL到API2
func PushToAPI2(apiURL string, orderNumbers []string, fileURL string) (string, error) {
	// 将单号数组转换为逗号分隔的字符串
	orderNumbersStr := strings.Join(orderNumbers, ",")

	// 构建请求URL
	requestURL := fmt.Sprintf("%s?orderNumbers=%s&fileURL=%s", apiURL, orderNumbersStr, url.QueryEscape(fileURL))

	// 设置超时时间
	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	// 发送GET请求
	resp, err := client.Get(requestURL)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	return string(body), nil
}

// UploadImages 根据配置上传本地路径中的所有图片到 MinIO
func UploadImages(config *Config) error {
	return UploadImagesWithTaskType(config, true) // 默认为计划任务
}

// UploadImagesWithTaskType 根据配置上传本地路径中的所有图片到 MinIO，指定任务类型
func UploadImagesWithTaskType(config *Config, isScheduledTask bool) error {
	// 检查本地路径是否存在图片文件
	hasImages, err := checkForImages(config.LocalFolder)
	if err != nil {
		return fmt.Errorf("检查图片文件失败: %v", err)
	}

	if !hasImages {
		return fmt.Errorf("无文件可上传")
	}

	// 初始化 MinIO 客户端
	client, err := InitMinioClient(config, config.UseSSL)
	if err != nil {
		return fmt.Errorf("初始化 MinIO 客户端失败: %v", err)
	}

	// 测试 MinIO 连接
	if err := TestConnection(client); err != nil {
		return fmt.Errorf("MinIO 连接测试失败: %v", err)
	}

	// 使用配置中的 machine_code 作为 MinIO 中的根路径
	machineCode := config.MachineCode
	if machineCode == "" {
		return fmt.Errorf("配置中的 machine_code 不能为空")
	}

	// 上传本地路径中的所有图片到 MinIO
	localPath := config.LocalFolder // 本地路径
	minioPath := machineCode        // MinIO 中的目标路径
	bucketName := config.BucketName // MinIO 中的存储桶名称
	api1URL := config.API1          // 配置中的 API1 URL
	api2URL := config.API2          // 配置中的 API2 URL

	// 在上传过程中记录日志
	logUploadMessage(fmt.Sprintf("开始上传图片，本地路径: %s, MinIO路径: %s", localPath, minioPath), isScheduledTask)

	// 上传图片并将日志指向正确的日志文件
	if err := UploadImagesToMinio(client, bucketName, localPath, minioPath, api1URL, api2URL, isScheduledTask); err != nil {
		return fmt.Errorf("上传图片失败: %v", err)
	}

	logUploadMessage("所有图片上传完成", isScheduledTask)
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
