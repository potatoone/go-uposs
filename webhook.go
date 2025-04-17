package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// NotifyUploadFailed 发送上传失败通知到企业微信
func (config *Config) NotifyUploadFailed() error {
	content := fmt.Sprintf(
		"图片第二次上传失败😭\n"+
			">存储桶:<font color=\"warning\"> %s</font>\n"+
			">机器代号:<font color=\"warning\"> %s</font>",
		config.BucketName, config.MachineCode,
	)

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("编码 JSON 失败: %w", err)
	}

	// 打印 webhook 地址（可选，调试时启用）
	fmt.Println("上传失败，发送企业微信通知...")

	resp, err := http.Post(config.WebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("请求 webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("编码 JSON 失败: %w", err)
	}

	return nil
}
