package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"go-uposs/internal/utils" // 导入 utils 包
)

// maxFileConcurrency 复制、压缩、上传文件时允许的最大并发数量
const maxFileConcurrency = 32

// 复制缓冲区大小的取值范围（配置与界面均以 KB 为单位）
// 每个并发复制协程都会各自申请一块该大小的缓冲区，
// 因此上限必须收紧，避免「配置值 × 并发数」把内存吃满。
const (
	minIOBufferKB     = 4    // 最小 4 KB
	maxIOBufferKB     = 8192 // 最大 8 MB（单协程）
	defaultIOBufferKB = 1024 // 默认 1 MB
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

	// HeartbeatURL 心跳服务器地址
	HeartbeatURL string `json:"heartbeat_url"`
	// HeartbeatInterval 心跳间隔（秒）
	HeartbeatInterval string `json:"heartbeat_interval"`

	// NTPServer 网络时间服务器地址（用于检测本机时间偏差）
	NTPServer string `json:"ntp_server"`
	// NTPInterval 时间检测间隔（秒）
	NTPInterval string `json:"ntp_interval"`
	// NTPThreshold 本机时间偏差告警阈值（秒）
	NTPThreshold string `json:"ntp_threshold"`

	CleanStartTime string `json:"cleaStartTime"`
	CleanEndTime   string `json:"cleanEndTime"`

	// AutoStart 是否开机自启动
	AutoStart string `json:"autostart"`

	// AutoStartAutoTask 是否在开机启动后自动执行AutoTask任务
	AutoStartAutoTask string `json:"autostart_auto"`

	// LockUI 是否锁定界面
	LockUI string `json:"lockui"`

	// CPUThreads 程序可占用的 CPU 线程数，超出 CPU 逻辑核心数时按逻辑核心数处理
	CPUThreads int `json:"cpu_threads"`

	// CopyConcurrency 批量复制文件的并发数量上限
	CopyConcurrency int `json:"copy_concurrency"`

	// CompressConcurrency 批量压缩文件的并发数量上限
	CompressConcurrency int `json:"compress_concurrency"`

	// UploadConcurrency 批量上传文件的并发数量上限
	UploadConcurrency int `json:"upload_concurrency"`
}

// normalizeFileConcurrency 将文件并发数限制在 1 到 maxFileConcurrency 之间
func normalizeFileConcurrency(concurrency int) int {
	if concurrency < 1 {
		return 1
	}
	if concurrency > maxFileConcurrency {
		return maxFileConcurrency
	}
	return concurrency
}

// NormalizePerformanceConfig 校正 CPU 线程数、文件并发数，并填充默认值
// 默认值：CPU 线程数取 CPU 逻辑核心数，复制/压缩/上传并发数均取 1（与历史串行行为一致）
func (config *Config) NormalizePerformanceConfig() {
	cpuCores := runtime.NumCPU()

	if config.CPUThreads <= 0 || config.CPUThreads > cpuCores {
		config.CPUThreads = cpuCores
	}

	config.CopyConcurrency = normalizeFileConcurrency(config.CopyConcurrency)
	config.CompressConcurrency = normalizeFileConcurrency(config.CompressConcurrency)
	config.UploadConcurrency = normalizeFileConcurrency(config.UploadConcurrency)
}

// ApplyRuntimeLimits 将 CPU 线程占用配置应用到 Go 运行时
func (config *Config) ApplyRuntimeLimits() {
	config.NormalizePerformanceConfig()
	runtime.GOMAXPROCS(config.CPUThreads)
}

// NormalizeIOBuffer 校正复制缓冲区大小（输入单位为 KB）
// 每个并发复制协程各持一块缓冲区，取值过大会成倍放大内存占用，因此这里做上下限约束
func normalizeIOBufferKB(kb int) int {
	if kb <= 0 {
		return defaultIOBufferKB
	}
	if kb < minIOBufferKB {
		return minIOBufferKB
	}
	if kb > maxIOBufferKB {
		return maxIOBufferKB
	}
	return kb
}

// NormalizeIOBuffer 按 KB 为单位校正并换算缓冲区字节数
func (config *Config) NormalizeIOBuffer() {
	// 此处 IOBuffer 仍为配置中的 KB 值
	if config.IOBuffer <= 0 {
		config.IOBuffer = defaultIOBufferKB
	}
	config.IOBuffer = normalizeIOBufferKB(config.IOBuffer) * 1024
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

	// 校正缓冲区大小（KB -> 字节，并限制在合理范围内）
	config.NormalizeIOBuffer()

	// 校正新增的并发配置，未配置时使用默认值
	config.NormalizePerformanceConfig()

	// 校正时间同步配置，未配置时使用默认值
	config.NormalizeTimeSyncConfig()

	return config, nil
}

// SaveConfig 保存格式化的 JSON 配置到文件
func SaveConfig(filename string, config *Config) error {
	// 使用 utils.DataPath 构建完整的配置文件路径
	configFilePath := filepath.Join(utils.DataPath, filename)

	// 将缓冲区大小从字节转换为KB，并校正到合法范围
	config.IOBuffer = normalizeIOBufferKB(config.IOBuffer / 1024)

	// 校正并发配置，避免写入非法值
	config.NormalizePerformanceConfig()

	data, err := json.MarshalIndent(config, "", "  ") // 美化 JSON
	if err != nil {
		return err
	}

	// 恢复为字节，供运行时使用
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
