package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go-uposs/utils" // 导入 utils 包
)

// Config 配置结构体
type Config struct {
	MachineCode     string `json:"machine_code"`
	BucketName      string `json:"bucket_name"`
	Endpoint        string `json:"endpoint"`   // 端点
	PublicUrl       string `json:"public_url"` // 互联网端点
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`

	UseSSL bool `json:"useSSL"`

	LocalFolder  string `json:"local_folder"`  // 复制到本地的路径
	RemoteFolder string `json:"remote_folder"` // 源获取路径

	PicCompress string `json:"pic_compress"` // 图片压缩比率
	PicWidth    string `json:"pic_width"`    // 图片宽度
	PicSize     int    `json:"pic_size"`     // 图片体积过滤，单位KB

	StartTime string `json:"start_time"` // 开始时间
	EndTime   string `json:"end_time"`   // 结束时间

	IOBuffer int `json:"io_buffer"` // 缓冲区大小，以KB为单位

	AutoInterval string `json:"auto_interval"` // 自动间隔时间
	SchedTimes   string `json:"sched_times"`   // 计划任务执行次数

	API1          string `json:"api1"`           // API1 URL
	API2          string `json:"api2"`           // API2 URL
	API1Response1 string `json:"api1_response1"` // API1 编号查询有效响应
	API1Response2 string `json:"api1_response2"` // API1 编号查询无效响应
	WebhookURL    string `json:"webhook_url"`    // 企业微信Webhook URL

	CleanStartTime string `json:"cleaStartTime"`
	CleanEndTime   string `json:"cleanEndTime"`

	// AutoStart 是否开机自启动
	AutoStart string `json:"autostart"`

	// AutoStartAutoTask 是否在开机启动后自动执行AutoTask任务
	AutoStartAutoTask string `json:"autostart_auto"`

	// LockUI 是否锁定界面
	LockUI string `json:"lockui"`
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	// 使用 utils.DataPath 构建完整的配置文件路径
	configFilePath := filepath.Join(utils.DataPath, filename)

	file, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	if err := json.NewDecoder(file).Decode(config); err != nil {
		return nil, err
	}

	// 将缓冲区大小从KB转换为字节
	config.IOBuffer *= 1024

	return config, nil
}

// SaveConfig 保存格式化的 JSON 配置到文件
func SaveConfig(filename string, config *Config) error {
	// 使用 utils.DataPath 构建完整的配置文件路径
	configFilePath := filepath.Join(utils.DataPath, filename)

	// 将缓冲区大小从字节转换为KB
	config.IOBuffer /= 1024

	data, err := json.MarshalIndent(config, "", "  ") // 美化 JSON
	if err != nil {
		return err
	}

	// 将缓冲区大小从KB转换为字节
	config.IOBuffer *= 1024

	return os.WriteFile(configFilePath, data, 0644) // 直接写入文件
}

// UpdatePicConfig 更新图片相关的配置（压缩比率和宽度）
func (config *Config) UpdatePicConfig(compress, width string, size int) {
	config.PicCompress = compress
	config.PicWidth = width
	config.PicSize = size

	// 更新配置文件
	if err := SaveConfig("config.json", config); err != nil { // 使用相对路径
		fmt.Printf("更新图片配置失败: %v\n", err)
	}
}

// UpdateTimeRange 更新计划任务开始时间和结束时间
func (config *Config) UpdateTimeRange(startTime, endTime string) {
	config.StartTime = startTime
	config.EndTime = endTime

	// 更新配置文件
	if err := SaveConfig("config.json", config); err != nil { // 使用相对路径
		fmt.Printf("更新时间范围失败: %v\n", err)
	}
}

// UpdateTimeRange 更新清理开始时间和结束时间
func (config *Config) UpdateCleanTimeRange(cleanStartTime, cleanEndTime string) {
	config.CleanStartTime = cleanStartTime
	config.CleanEndTime = cleanEndTime

	// 更新配置文件
	if err := SaveConfig("config.json", config); err != nil { // 使用相对路径
		fmt.Printf("更新时间范围失败: %v\n", err)
	}
}
