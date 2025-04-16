package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// NotifyUploadFailed 发送上传失败通知到企业微信
func (config *Config) NotifyUploadFailed() error {
	content := fmt.Sprintf("📡 上传失败！\n机器码: %s\n桶名称: %s", config.MachineCode, config.BucketName)

	payload := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("编码 JSON 失败: %w", err)
	}

	resp, err := http.Post(config.WebhookURL, "application/json", bytes.NewBuffer(data)) // ✅ 使用配置中的 URL
	if err != nil {
		return fmt.Errorf("请求 webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Webhook 返回状态码异常: %d", resp.StatusCode)
	}

	return nil
}
