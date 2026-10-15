package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// ==================== 系统信息数据结构 ====================

// CPUInfo CPU 信息
type CPUInfo struct {
	Usage    float64 `json:"usage"`    // CPU 使用率 %
	Cores    int     `json:"cores"`    // 逻辑核心数
	Model    string  `json:"model"`    // CPU 型号
	Arch     string  `json:"arch"`     // 架构
	OS       string  `json:"os"`       // 操作系统版本
	Hostname string  `json:"hostname"` // 主机名
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total     uint64  `json:"total"`     // 总内存（字节）
	Available uint64  `json:"available"` // 可用内存（字节）
	Used      uint64  `json:"used"`      // 已用内存（字节）
	Usage     float64 `json:"usage"`     // 内存使用率 %
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Drive string  `json:"drive"` // 盘符，如 C:
	Total uint64  `json:"total"` // 总容量（字节）
	Free  uint64  `json:"free"`  // 可用空间（字节）
	Used  uint64  `json:"used"`  // 已用空间（字节）
	Usage float64 `json:"usage"` // 使用率 %
}

// SystemInfo 系统信息汇总
type SystemInfo struct {
	MachineCode string     `json:"machine_code"` // 机器码
	Timestamp   string     `json:"timestamp"`    // 采集时间
	CPU         CPUInfo    `json:"cpu"`
	Memory      MemoryInfo `json:"memory"`
	Disks       []DiskInfo `json:"disks"`
}

// ==================== Windows API 声明 ====================

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetLogicalDrives     = kernel32.NewProc("GetLogicalDrives")
	procGetNativeSystemInfo  = kernel32.NewProc("GetNativeSystemInfo")
)

// filetime Windows FILETIME 结构
type filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

// memoryStatusEx Windows MEMORYSTATUSEX 结构
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// systemInfo Windows SYSTEM_INFO 结构
type systemInfo struct {
	OemID                     uint32
	ProcessorArchitecture     uint16
	Reserved                  uint16
	PageSize                  uint32
	MinimumApplicationAddress uintptr
	MaximumApplicationAddress uintptr
	ActiveProcessorMask       uintptr
	NumberOfProcessors        uint32
	ProcessorType             uint32
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

// ==================== CPU 信息 ====================

// getSystemTimes 获取系统时间（空闲/内核/用户）
func getSystemTimes() (idle, kernel, user filetime) {
	procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	return
}

// filetimeToInt64 将 FILETIME 转换为纳秒
func filetimeToInt64(ft filetime) int64 {
	return int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
}

// getCPUUsage 采样获取 CPU 使用率（%）
func getCPUUsage() float64 {
	// 第一次采样
	idle1, kernel1, user1 := getSystemTimes()

	// 等待 500ms 后第二次采样
	time.Sleep(500 * time.Millisecond)

	idle2, kernel2, user2 := getSystemTimes()

	idleDelta := filetimeToInt64(idle2) - filetimeToInt64(idle1)
	kernelDelta := filetimeToInt64(kernel2) - filetimeToInt64(kernel1)
	userDelta := filetimeToInt64(user2) - filetimeToInt64(user1)

	// 内核时间包含空闲时间
	total := kernelDelta + userDelta
	if total <= 0 {
		return 0
	}

	usage := float64(total-idleDelta) / float64(total) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	return usage
}

// getCPUModel 从注册表获取 CPU 型号
func getCPUModel() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	model, _, err := key.GetStringValue("ProcessorNameString")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(model)
}

// getLogicalCores 获取逻辑核心数
func getLogicalCores() int {
	var si systemInfo
	procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&si)))
	if si.NumberOfProcessors == 0 {
		return runtime.NumCPU()
	}
	return int(si.NumberOfProcessors)
}

// ==================== 内存信息 ====================

// getMemoryInfo 获取内存信息
func getMemoryInfo() MemoryInfo {
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))

	info := MemoryInfo{
		Total:     ms.TotalPhys,
		Available: ms.AvailPhys,
	}
	if info.Total > info.Available {
		info.Used = info.Total - info.Available
	}
	if info.Total > 0 {
		info.Usage = float64(info.Used) / float64(info.Total) * 100
	}
	return info
}

// ==================== 磁盘信息 ====================

// getDiskInfo 枚举所有磁盘分区信息
func getDiskInfo() []DiskInfo {
	var disks []DiskInfo

	// 获取可用驱动器位掩码
	drives, _, _ := procGetLogicalDrives.Call()
	if drives == 0 {
		return disks
	}

	for i := 0; i < 26; i++ {
		if drives&(1<<uint(i)) == 0 {
			continue
		}
		drive := string(rune('A'+i)) + ":\\"

		var freeBytesAvailable, totalBytes, totalFreeBytes uint64
		ptr, err := syscall.UTF16PtrFromString(drive)
		if err != nil {
			continue
		}

		ret, _, _ := procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(ptr)),
			uintptr(unsafe.Pointer(&freeBytesAvailable)),
			uintptr(unsafe.Pointer(&totalBytes)),
			uintptr(unsafe.Pointer(&totalFreeBytes)),
		)
		if ret == 0 {
			continue
		}

		disk := DiskInfo{
			Drive: drive,
			Total: totalBytes,
			Free:  totalFreeBytes,
		}
		if disk.Total > disk.Free {
			disk.Used = disk.Total - disk.Free
		}
		if disk.Total > 0 {
			disk.Usage = float64(disk.Used) / float64(disk.Total) * 100
		}
		disks = append(disks, disk)
	}
	return disks
}

// ==================== 汇总与格式化 ====================

// CollectSystemInfo 采集完整的系统信息
func CollectSystemInfo() *SystemInfo {
	hostname, _ := os.Hostname()

	info := &SystemInfo{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		CPU: CPUInfo{
			Usage:    getCPUUsage(),
			Cores:    getLogicalCores(),
			Model:    getCPUModel(),
			Arch:     runtime.GOARCH,
			OS:       runtime.GOOS,
			Hostname: hostname,
		},
		Memory: getMemoryInfo(),
		Disks:  getDiskInfo(),
	}
	return info
}

// FormatBytes 将字节数格式化为可读字符串（B/KB/MB/GB/TB）
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatSystemInfo 将系统信息格式化为多行可读文本
func FormatSystemInfo(info *SystemInfo) string {
	if info == nil {
		return "暂无系统信息"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("主机名: %s\n", info.CPU.Hostname))
	sb.WriteString(fmt.Sprintf("系统: %s (%s)\n", info.CPU.OS, info.CPU.Arch))
	if info.CPU.Model != "" {
		sb.WriteString(fmt.Sprintf("CPU型号: %s\n", info.CPU.Model))
	}
	sb.WriteString(fmt.Sprintf("CPU核心数: %d\n", info.CPU.Cores))
	sb.WriteString(fmt.Sprintf("CPU使用率: %.1f%%\n", info.CPU.Usage))
	sb.WriteString(fmt.Sprintf("内存: 总 %s, 已用 %s, 可用 %s, 使用率 %.1f%%\n",
		FormatBytes(info.Memory.Total),
		FormatBytes(info.Memory.Used),
		FormatBytes(info.Memory.Available),
		info.Memory.Usage))
	for _, d := range info.Disks {
		sb.WriteString(fmt.Sprintf("磁盘 %s: 总 %s, 已用 %s, 可用 %s, 使用率 %.1f%%\n",
			d.Drive,
			FormatBytes(d.Total),
			FormatBytes(d.Used),
			FormatBytes(d.Free),
			d.Usage))
	}
	return sb.String()
}

// ==================== 心跳 ====================

// NormalizeURL 确保 URL 带有协议前缀
// 未指定协议时：本地地址（127.0.0.1/localhost/内网IP）使用 http://，其余默认 https://
func NormalizeURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return rawURL
	}

	// 已带协议前缀则直接返回
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}

	// 判断是否为本地/内网地址，使用 http 协议
	isLocal := strings.HasPrefix(rawURL, "127.") ||
		strings.HasPrefix(rawURL, "localhost") ||
		strings.HasPrefix(rawURL, "10.") ||
		strings.HasPrefix(rawURL, "192.168.") ||
		strings.HasPrefix(rawURL, "172.16.") ||
		strings.HasPrefix(rawURL, "172.17.") ||
		strings.HasPrefix(rawURL, "172.18.") ||
		strings.HasPrefix(rawURL, "172.19.") ||
		strings.HasPrefix(rawURL, "172.2") ||
		strings.HasPrefix(rawURL, "172.3") ||
		strings.HasPrefix(rawURL, "0.0.0.0") ||
		strings.HasPrefix(rawURL, "[::1]")

	if isLocal {
		return "http://" + rawURL
	}
	return "https://" + rawURL
}

// StripURLScheme 去除 URL 中的协议前缀和路径，返回 host:port 形式的地址（用于 UDP）
func StripURLScheme(rawURL string) string {
	s := strings.TrimSpace(rawURL)
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "udp://")

	// 去掉路径部分（如 /heartbeat）
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// HeartbeatPayload 心跳数据载荷
type HeartbeatPayload struct {
	MachineCode string     `json:"machine_code"` // 机器码
	Timestamp   string     `json:"timestamp"`    // 心跳时间
	CPU         CPUInfo    `json:"cpu"`
	Memory      MemoryInfo `json:"memory"`
	Disks       []DiskInfo `json:"disks"`
}

// SendHeartbeat 向服务器发送心跳，返回服务器响应内容
func SendHeartbeat(serverURL, machineCode string, info *SystemInfo) (string, error) {
	if serverURL == "" {
		return "", fmt.Errorf("心跳地址为空")
	}

	// 确保 URL 有正确的协议前缀
	serverURL = NormalizeURL(serverURL)

	if info == nil {
		info = CollectSystemInfo()
	}

	// 构建心跳载荷
	payload := HeartbeatPayload{
		MachineCode: machineCode,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
		CPU:         info.CPU,
		Memory:      info.Memory,
		Disks:       info.Disks,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("JSON编码失败: %v", err)
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 创建 POST 请求
	req, err := http.NewRequest("POST", serverURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 GoUpOSS Client")

	// 发送请求
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

	// 截断过长响应
	bodyStr := string(body)
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500] + "..."
	}

	return fmt.Sprintf("状态码:%d 响应:%s", resp.StatusCode, bodyStr), nil
}

// SendHeartbeatUDP 通过 UDP 发送心跳数据（无需回包，fire-and-forget）
func SendHeartbeatUDP(serverAddr, machineCode string, info *SystemInfo) error {
	if serverAddr == "" {
		return fmt.Errorf("心跳地址为空")
	}

	// UDP 地址为 host:port 格式，去除可能的协议前缀和路径
	serverAddr = StripURLScheme(serverAddr)

	if info == nil {
		info = CollectSystemInfo()
	}

	// 构建心跳载荷
	payload := HeartbeatPayload{
		MachineCode: machineCode,
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
		CPU:         info.CPU,
		Memory:      info.Memory,
		Disks:       info.Disks,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	// 建立 UDP 连接并发送数据（UDP 无需等待回包）
	conn, err := net.Dial("udp", serverAddr)
	if err != nil {
		return fmt.Errorf("UDP连接失败: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write(jsonData); err != nil {
		return fmt.Errorf("UDP发送失败: %v", err)
	}
	return nil
}

// ==================== 本程序资源占用 ====================

// ProcessUsage 本程序资源占用
type ProcessUsage struct {
	CPUPercent  float64 `json:"cpu_percent"`  // CPU 占用（按逻辑核心归一化，0-100）
	MemoryBytes uint64  `json:"memory_bytes"` // 物理内存占用（工作集，字节）
	GoVersion   string  `json:"go_version"`   // 运行环境 Go 版本
}

var (
	psapi = syscall.NewLazyDLL("psapi.dll")

	procGetCurrentProcess    = kernel32.NewProc("GetCurrentProcess")
	procGetProcessTimes      = kernel32.NewProc("GetProcessTimes")
	procGetProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")

	processUsageOnce sync.Once
	processUsageMu   sync.RWMutex
	processUsageSnap = ProcessUsage{GoVersion: runtime.Version()}
)

// processMemoryCounters Windows PROCESS_MEMORY_COUNTERS 结构
type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// StartProcessUsageSampler 启动本程序 CPU/内存占用采样，重复调用只会启动一次
// interval 为采样间隔，小于等于 0 时按 2 秒处理
func StartProcessUsageSampler(interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}

	processUsageOnce.Do(func() {
		go func() {
			// 先取一次内存占用，避免界面刚打开时显示为 0
			processUsageMu.Lock()
			processUsageSnap.MemoryBytes = processWorkingSet()
			processUsageMu.Unlock()

			prevCPU, prevAt := processCPUTime(), time.Now()

			for {
				time.Sleep(interval)

				now := time.Now()
				cpuTime := processCPUTime()
				elapsed := now.Sub(prevAt).Seconds()

				percent := 0.0
				if elapsed > 0 && cpuTime > prevCPU {
					// FILETIME 单位为 100 纳秒，除以 1e7 得到秒数，再按逻辑核心数归一化
					percent = float64(cpuTime-prevCPU) / 1e7 / elapsed / float64(runtime.NumCPU()) * 100
				}
				if percent < 0 {
					percent = 0
				}
				if percent > 100 {
					percent = 100
				}
				prevCPU, prevAt = cpuTime, now

				processUsageMu.Lock()
				processUsageSnap.CPUPercent = percent
				processUsageSnap.MemoryBytes = processWorkingSet()
				processUsageSnap.GoVersion = runtime.Version()
				processUsageMu.Unlock()
			}
		}()
	})
}

// GetProcessUsage 读取最近一次采样的本程序资源占用
func GetProcessUsage() ProcessUsage {
	processUsageMu.RLock()
	defer processUsageMu.RUnlock()

	usage := processUsageSnap
	usage.GoVersion = runtime.Version()
	return usage
}

// processCPUTime 获取本程序累计占用的 CPU 时间（100 纳秒）
func processCPUTime() uint64 {
	var creation, exit, kernel, user filetime

	handle, _, _ := procGetCurrentProcess.Call()
	ret, _, _ := procGetProcessTimes.Call(handle,
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)))
	if ret == 0 {
		return 0
	}

	return uint64(filetimeToInt64(kernel) + filetimeToInt64(user))
}

// processWorkingSet 获取本程序当前占用的物理内存（工作集，字节）
func processWorkingSet() uint64 {
	var counters processMemoryCounters
	counters.CB = uint32(unsafe.Sizeof(counters))

	handle, _, _ := procGetCurrentProcess.Call()
	ret, _, _ := procGetProcessMemoryInfo.Call(handle,
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.CB))
	if ret == 0 {
		return 0
	}

	return uint64(counters.WorkingSetSize)
}
