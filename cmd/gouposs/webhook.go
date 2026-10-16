package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NotifyUploadFailed 发送上传失败通知到企业微信
// 同类通知在冷却时间内只发送一次，被抑制的次数会在下次实际发送时告知，避免持续故障刷屏
// isScheduledTask 仅用于把抑制记录写入对应的任务日志（true=计划任务，false=自动任务）
func (config *Config) NotifyUploadFailed(isScheduledTask bool) error {
	allowed, suppressed, firstBlocked := uploadNotifyState.allow(notifyKindUploadFailed, time.Now())
	if !allowed {
		logTaskMessage(fmt.Sprintf("上传失败通知处于 %v 冷却期内，本次已抑制（本周期累计抑制 %d 次）",
			notifyCooldown, suppressed), isScheduledTask)
		return nil
	}

	content := fmt.Sprintf(
		"图片第二次上传失败❌😅\n"+
			">存储桶:<font color=\"warning\"> %s</font>\n"+
			">机器代号:<font color=\"warning\"> %s</font>",
		config.BucketName, config.MachineCode,
	)

	// 冷却期内被抑制过：补充说明，便于判断故障持续时间
	if suppressed > 0 {
		content += fmt.Sprintf("\n>另有 %d 次失败在 %v 冷却期内被抑制（首次 %s）",
			suppressed, notifyCooldown, firstBlocked.Format("2006-01-02 15:04:05"))
	}

	// 打印 webhook 地址（可选，调试时启用）
	fmt.Println("上传失败，发送企业微信通知...")

	return config.SendWebhook(content)
}

// MarkUploadRecovered 上传恢复正常时调用
// 仅当期间确实抑制过通知时才清空冷却状态，使下次故障能立即通知
func MarkUploadRecovered(isScheduledTask bool) {
	if uploadNotifyState.resetIfPending(notifyKindUploadFailed) {
		logTaskMessage("上传已恢复正常，上传失败通知的抑制记录已清空，下次失败将立即通知", isScheduledTask)
	}
}

// logTaskMessage 按任务类型写入对应的任务日志
func logTaskMessage(message string, isScheduledTask bool) {
	if isScheduledTask {
		SchedLogToFile(message)
		return
	}
	AutoLogToFile(message)
}

// NotifyTimeOffset 发送本机时间偏差通知到企业微信
func (config *Config) NotifyTimeOffset(server, localTime, remoteTime string, offsetSeconds float64, thresholdSeconds int) error {
	content := fmt.Sprintf(
		"本机时间偏差过大⚠️\n"+
			">机器代号:<font color=\"warning\"> %s</font>\n"+
			">NTP 服务器:<font color=\"comment\"> %s</font>\n"+
			">网络时间:<font color=\"comment\"> %s</font>\n"+
			">本机时间:<font color=\"warning\"> %s</font>\n"+
			">偏差:<font color=\"warning\"> %+.3f 秒</font>（阈值 %d 秒）\n"+
			">可能导致 MinIO 上传签名校验失败，请校准系统时间",
		config.MachineCode, server, remoteTime, localTime, offsetSeconds, thresholdSeconds,
	)

	fmt.Println("本机时间偏差过大，发送企业微信通知...")

	return config.SendWebhook(content)
}

// SendWebhook 向企业微信机器人 webhook 推送 markdown 消息
func (config *Config) SendWebhook(content string) error {
	if config.WebhookURL == "" {
		return fmt.Errorf("未配置 webhook 地址")
	}

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

	resp, err := http.Post(config.WebhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("请求 webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("请求 webhook 返回状态码: %d", resp.StatusCode)
	}

	return nil
}
