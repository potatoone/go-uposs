package main

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioLogToFile 记录 MinIO 相关日志
func MinioLogToFile(message string) {
	// 不在这里添加时间戳，只添加标识符
	formattedMessage := fmt.Sprintf("[MinIO] %s", message)

	// 写入系统日志，SysLogToFile 会添加时间戳
	SysLogToFile(formattedMessage)

	// 获取当前时间用于 UI 显示
	currentTime := time.Now().Format("2006-01-02 15:04:05")

	// 为 UI 日志添加时间戳
	uiMessage := fmt.Sprintf("[%s] %s", currentTime, formattedMessage)

	logText.SetText(uiMessage + "\n" + logText.Text)

}

// InitMinioClient 初始化 MinIO 客户端
func InitMinioClient(config *Config, useSSL bool) (*minio.Client, error) {
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化客户端失败: %v", err)
	}

	logMessage := fmt.Sprintf("客户端初始化成功 | MachineCode: %s | Endpoint: %s | UseSSL: %v",
		config.MachineCode, config.Endpoint, useSSL)
	MinioLogToFile(logMessage)

	return client, nil
}

// TestConnection 测试 MinIO 连接（带超时）
func TestConnection(client *minio.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("MinIO 连接测试失败: %v", err)
	}
	return nil
}

// TestMinioConnection 加载配置、初始化客户端、测试连接
func TestMinioConnection(useSSL bool) (string, error) {
	// 加载配置
	config, err := LoadConfig("config.json")

	if err != nil {
		MinioLogToFile(fmt.Sprintf("加载配置失败: %v", err))
		return "", fmt.Errorf("加载配置失败: %v", err)
	}

	MinioLogToFile(fmt.Sprintf("测试与 %s 的连接 (UseSSL: %t)", config.Endpoint, useSSL))

	// 初始化 MinIO 客户端
	client, err := InitMinioClient(config, useSSL)
	if err != nil {
		MinioLogToFile(fmt.Sprintf("初始化MinIO客户端失败: %v", err))
		return "", err
	}

	// 测试连接
	if err := TestConnection(client); err != nil {
		MinioLogToFile(fmt.Sprintf("连接测试失败: %v", err))
		return "", err
	}

	// 获取存储桶列表
	buckets, err := client.ListBuckets(context.Background())
	if err != nil {
		MinioLogToFile(fmt.Sprintf("获取存储桶列表失败: %v", err))
		return "", fmt.Errorf("获取存储桶列表失败: %v", err)
	}

	// 将存储桶列表写入日志
	MinioLogToFile(fmt.Sprintf("连接成功! 找到 %d 个存储桶", len(buckets)))

	// 记录每个存储桶的详细信息到日志
	for _, bucket := range buckets {
		MinioLogToFile(fmt.Sprintf("存储桶: %s (创建于: %s)", bucket.Name, bucket.CreationDate.Format("2006-01-02 15:04:05")))
	}

	return "连接测试成功", nil
}

// ListBuckets 获取存储桶列表
func ListBuckets(client *minio.Client) ([]minio.BucketInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		MinioLogToFile(fmt.Sprintf("获取存储桶列表失败: %v", err))
		return nil, fmt.Errorf("获取存储桶列表失败: %v", err)
	}

	MinioLogToFile(fmt.Sprintf("成功获取 %d 个存储桶信息", len(buckets)))
	return buckets, nil
}
