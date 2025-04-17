package main

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

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
	updateLog(ossLogText, "[MinioClient]", logMessage)

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
		// 记录加载配置失败
		updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("加载配置失败: %v", err)) // 更新日志
		return "", fmt.Errorf("加载配置失败: %v", err)
	}

	// 记录连接测试开始
	updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("测试与 %s 的连接 (UseSSL: %t)", config.Endpoint, useSSL)) // 更新日志

	// 初始化 MinIO 客户端
	client, err := InitMinioClient(config, useSSL)
	if err != nil {
		// 记录初始化失败
		updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("初始化MinIO客户端失败: %v", err)) // 更新日志
		return "", err
	}

	// 测试连接
	if err := TestConnection(client); err != nil {
		updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("连接测试失败: %v", err)) // 更新日志
		return "", err
	}

	// 获取存储桶列表
	buckets, err := client.ListBuckets(context.Background())
	if err != nil {
		updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("获取存储桶列表失败: %v", err)) // 更新日志
		return "", fmt.Errorf("获取存储桶列表失败: %v", err)
	}

	// 记录连接成功并找到的存储桶数量
	updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("连接成功! 找到 %d 个存储桶", len(buckets))) // 更新日志

	// 记录每个存储桶的详细信息
	for _, bucket := range buckets {
		updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("存储桶: %s (创建于: %s)", bucket.Name, bucket.CreationDate.Format("2006-01-02 15:04:05"))) // 更新日志
	}

	return "连接测试成功", nil
}

// ListBuckets 获取存储桶列表
func ListBuckets(client *minio.Client) ([]minio.BucketInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("获取存储桶列表失败: %v", err)) // 更新日志
		return nil, fmt.Errorf("获取存储桶列表失败: %v", err)
	}

	updateLog(ossLogText, "[MinioClient]", fmt.Sprintf("成功获取 %d 个存储桶信息", len(buckets))) // 更新日志
	return buckets, nil
}
