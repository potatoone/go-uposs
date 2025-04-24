package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// QueryAPI1 调用API1进行编号验证
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

// PushToAPI2 推送编号和文件URL到API2，使用POST请求和JSON格式
func PushToAPI2(apiURL string, orderNumber string, fileUrl string) (string, error) {
	// 创建请求体数据结构
	requestData := struct {
		OrderNumber string `json:"orderNumber"`
		FileUrl     string `json:"fileUrl"`
	}{
		OrderNumber: orderNumber,
		FileUrl:     fileUrl,
	}

	// 将数据结构转换为JSON
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %v", err)
	}

	// 设置超时时间
	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	// 创建POST请求
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头为JSON
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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
		Code      int         `json:"code"`
		Msg       string      `json:"msg"`
		Data      interface{} `json:"data"`
		Timestamp string      `json:"timestamp"`
		TraceId   interface{} `json:"traceId"`
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return string(body), fmt.Errorf("解析JSON响应失败: %v", err)
	}

	// 检查响应状态码
	if response.Code != 200 {
		return string(body), fmt.Errorf("API2响应状态码非200: %d, 消息: %s", response.Code, response.Msg)
	}

	return string(body), nil
}
