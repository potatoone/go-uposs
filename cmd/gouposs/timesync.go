package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// ==================== 时间同步（仅检测，不修改系统时间） ====================
//
// MinIO 使用 AWS Signature V4 签名，请求头里的时间由本机时钟决定，
// 本机时间偏差过大会导致签名校验失败（服务端默认允许约 15 分钟）。
// 这里通过 SNTP 查询网络时间，发现偏差超阈值时写系统日志并弹通知提醒。

const (
	ntpPort            = "123"            // SNTP 标准端口
	ntpEpochOffset     = 2208988800       // NTP 纪元(1900) 到 Unix 纪元(1970) 的秒数
	defaultNTPServer   = "ntp.aliyun.com" // 默认 NTP 服务器
	defaultNTPInterval = 3600             // 默认检测间隔（秒）
	defaultNTPWarn     = 5                // 默认偏差告警阈值（秒）
	ntpQueryTimeout    = 5 * time.Second  // 单次查询超时
)

// TimeSyncResult 一次时间检测的结果
type TimeSyncResult struct {
	Server     string        // 使用的 NTP 服务器
	CheckedAt  time.Time     // 检测时刻（本机时间）
	LocalTime  time.Time     // 本机时间
	RemoteTime time.Time     // 网络时间
	Offset     time.Duration // 网络时间 - 本机时间
	RTT        time.Duration // 往返延迟
}

// timeSyncState 时间同步运行状态
type timeSyncState struct {
	mu          sync.Mutex
	once        sync.Once
	last        TimeSyncResult
	lastErr     error
	warned      bool          // 已就本次偏差告警，回到阈值内后复位
	statusLabel *widget.Label // 监控页状态标签
}

var globalTimeSync = &timeSyncState{}

// queryNTPTime 通过 SNTP 查询网络时间，返回网络时间、与本机时间的偏差和往返延迟
// server 可以是主机名，也可以是 主机:端口（未指定端口时使用 123）
func queryNTPTime(server string, timeout time.Duration) (time.Time, time.Duration, time.Duration, error) {
	if server == "" {
		return time.Time{}, 0, 0, fmt.Errorf("NTP 服务器地址为空")
	}

	addr, err := net.ResolveUDPAddr("udp", ntpAddress(server))
	if err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("解析服务器地址失败: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("UDP 连接失败: %v", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("设置超时失败: %v", err)
	}

	// 请求包 48 字节：LI=0、VN=3、Mode=3(客户端)
	request := make([]byte, 48)
	request[0] = 0x1B

	t1 := time.Now()
	if _, err := conn.Write(request); err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("发送请求失败: %v", err)
	}

	response := make([]byte, 48)
	n, err := conn.Read(response)
	t4 := time.Now()
	if err != nil {
		return time.Time{}, 0, 0, fmt.Errorf("读取响应失败: %v", err)
	}
	if n < 48 {
		return time.Time{}, 0, 0, fmt.Errorf("响应长度不足: %d 字节", n)
	}

	mode := response[0] & 0x07
	if mode != 4 && mode != 5 { // 4=服务器, 5=广播
		return time.Time{}, 0, 0, fmt.Errorf("非预期的响应模式: %d", mode)
	}
	if stratum := response[1]; stratum == 0 {
		return time.Time{}, 0, 0, fmt.Errorf("服务器未同步（stratum 0），响应: %s", string(trimNul(response[12:16])))
	}

	t2 := ntpTimestampToTime(response[32:40]) // 服务器接收时刻
	t3 := ntpTimestampToTime(response[40:48]) // 服务器发送时刻
	if t3.IsZero() {
		return time.Time{}, 0, 0, fmt.Errorf("服务器返回的发送时刻无效")
	}

	// 偏差 = ((T2 - T1) + (T3 - T4)) / 2，正值表示本机时间偏慢
	offset := (t2.Sub(t1) + t3.Sub(t4)) / 2
	// 往返延迟 = (T4 - T1) - (T3 - T2)
	rtt := t4.Sub(t1) - t3.Sub(t2)

	return t3, offset, rtt, nil
}

// ntpAddress 补全 NTP 服务器端口，未指定端口时使用 123
func ntpAddress(server string) string {
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	return net.JoinHostPort(server, ntpPort)
}

// ntpTimestampToTime 将 NTP 时间戳（64 位，高 32 位秒、低 32 位小数）转换为 Unix 时间
func ntpTimestampToTime(b []byte) time.Time {
	seconds := binary.BigEndian.Uint32(b[0:4])
	fraction := binary.BigEndian.Uint32(b[4:8])
	if seconds == 0 {
		return time.Time{}
	}

	// 秒部分去掉 NTP 纪元与 Unix 纪元的差值，小数部分换算成纳秒
	unix := int64(seconds) - ntpEpochOffset
	nanos := int64((uint64(fraction) * 1e9) >> 32)
	return time.Unix(unix, nanos)
}

func trimNul(b []byte) []byte {
	for i, c := range b {
		if c == 0 {
			return b[:i]
		}
	}
	return b
}

// NormalizeTimeSyncConfig 校正时间同步配置，未配置时填充默认值
func (config *Config) NormalizeTimeSyncConfig() {
	if config.NTPServer == "" {
		config.NTPServer = defaultNTPServer
	}
	if _, err := strconv.Atoi(config.NTPInterval); err != nil {
		config.NTPInterval = strconv.Itoa(defaultNTPInterval)
	}
	if _, err := strconv.Atoi(config.NTPThreshold); err != nil {
		config.NTPThreshold = strconv.Itoa(defaultNTPWarn)
	}
}

// timeSyncInterval 解析检测间隔（秒），非法时使用默认值
func timeSyncInterval(config *Config) int {
	if n, err := strconv.Atoi(config.NTPInterval); err == nil && n > 0 {
		return n
	}
	return defaultNTPInterval
}

// timeSyncThreshold 解析偏差告警阈值（秒），非法时使用默认值
func timeSyncThreshold(config *Config) int {
	if n, err := strconv.Atoi(config.NTPThreshold); err == nil && n > 0 {
		return n
	}
	return defaultNTPWarn
}

// checkTimeOnce 执行一次时间检测：写日志，偏差超阈值时告警
func checkTimeOnce(config *Config) (TimeSyncResult, error) {
	server := config.NTPServer
	if server == "" {
		server = defaultNTPServer
	}

	checkedAt := time.Now()
	remote, offset, rtt, err := queryNTPTime(server, ntpQueryTimeout)
	result := TimeSyncResult{
		Server:     server,
		CheckedAt:  checkedAt,
		LocalTime:  checkedAt.Add(offset),
		RemoteTime: remote,
		Offset:     offset,
		RTT:        rtt,
	}

	if err != nil {
		message := fmt.Sprintf("[时间同步] 查询 %s 失败: %v", server, err)
		SysLogToFile(message)
		logHeartbeat(fmt.Sprintf("时间检测失败: %v", err))

		globalTimeSync.mu.Lock()
		globalTimeSync.last = result
		globalTimeSync.lastErr = err
		globalTimeSync.mu.Unlock()
		setTimeSyncStatus(fmt.Sprintf("检测失败: %v", err))
		return result, err
	}

	threshold := time.Duration(timeSyncThreshold(config)) * time.Second
	// 用本地时间显示偏差方向：正值表示本机时间偏慢
	message := fmt.Sprintf("[时间同步] 服务器 %s，网络时间 %s，本机时间 %s，偏差 %+.3f 秒，往返延迟 %d ms",
		server, remote.Format("2006-01-02 15:04:05"), checkedAt.Format("2006-01-02 15:04:05"),
		offset.Seconds(), rtt.Milliseconds())

	globalTimeSync.mu.Lock()
	globalTimeSync.last = result
	globalTimeSync.lastErr = nil
	needWarn := offset.Abs() > threshold && !globalTimeSync.warned
	recovered := offset.Abs() <= threshold && globalTimeSync.warned
	if offset.Abs() > threshold {
		globalTimeSync.warned = true
	} else {
		globalTimeSync.warned = false
	}
	globalTimeSync.mu.Unlock()

	setTimeSyncStatus(fmt.Sprintf("网络时间 %s　本机偏差 %+.3f 秒", remote.Format("15:04:05"), offset.Seconds()))

	if needWarn {
		warning := fmt.Sprintf("[时间同步] 本机时间与网络时间偏差 %+.3f 秒，超过阈值 %d 秒，可能导致 MinIO 上传签名校验失败，请校准系统时间",
			offset.Seconds(), timeSyncThreshold(config))
		SysLogToFile(warning)
		logHeartbeat(fmt.Sprintf("本机时间偏差 %+.3f 秒，超过阈值，可能导致 MinIO 上传失败", offset.Seconds()))
		sendTimeSyncNotification("系统时间偏差过大", fmt.Sprintf("本机时间与网络时间相差 %.1f 秒，可能导致上传失败，请校准系统时间",
			offset.Seconds()))

		// 通过企业微信 webhook 推送偏差告警
		go func(cfg *Config, result TimeSyncResult, threshold int) {
			err := cfg.NotifyTimeOffset(result.Server,
				result.CheckedAt.Format("2006-01-02 15:04:05"),
				result.RemoteTime.Format("2006-01-02 15:04:05"),
				result.Offset.Seconds(), threshold)
			if err != nil {
				SysLogToFile(fmt.Sprintf("[时间同步] 发送企业微信通知失败: %v", err))
				return
			}
			SysLogToFile("[时间同步] 已发送企业微信通知")
		}(config, result, timeSyncThreshold(config))
	} else {
		if recovered {
			SysLogToFile(fmt.Sprintf("[时间同步] 本机时间偏差已回到阈值内: %+.3f 秒", offset.Seconds()))
		}
		SysLogToFile(message)
		logHeartbeat(fmt.Sprintf("时间检测正常，偏差 %+.3f 秒（%d ms）", offset.Seconds(), rtt.Milliseconds()))
	}

	return result, nil
}

// sendTimeSyncNotification 发送系统通知，App 尚未创建时忽略
func sendTimeSyncNotification(title, content string) {
	myApp := fyne.CurrentApp()
	if myApp == nil {
		return
	}
	go myApp.SendNotification(fyne.NewNotification(title, content))
}

// setTimeSyncStatus 更新监控页的时间同步状态标签
func setTimeSyncStatus(text string) {
	globalTimeSync.mu.Lock()
	label := globalTimeSync.statusLabel
	globalTimeSync.mu.Unlock()

	if label == nil {
		return
	}
	fyne.Do(func() {
		label.SetText(text)
	})
}

// GetLastTimeSync 读取最近一次时间检测结果
func GetLastTimeSync() (TimeSyncResult, error) {
	globalTimeSync.mu.Lock()
	defer globalTimeSync.mu.Unlock()
	return globalTimeSync.last, globalTimeSync.lastErr
}

// StartTimeSyncLoop 启动后台时间检测循环（程序启动时自动调用，重复调用只启动一次）
func StartTimeSyncLoop(config *Config) {
	globalTimeSync.once.Do(func() {
		interval := timeSyncInterval(config)
		SysLogToFile(fmt.Sprintf("[时间同步] 已启动，服务器 %s，间隔 %d 秒，告警阈值 %d 秒",
			config.NTPServer, interval, timeSyncThreshold(config)))

		go func() {
			// 启动后先检测一次
			checkTimeOnce(config)

			for {
				time.Sleep(time.Duration(interval) * time.Second)

				// 每轮重新加载配置，允许运行期间调整服务器和间隔
				latest, err := LoadConfig("config.json")
				if err != nil {
					SysLogToFile(fmt.Sprintf("[时间同步] 加载配置失败: %v", err))
					continue
				}
				if newInterval := timeSyncInterval(latest); newInterval != interval {
					interval = newInterval
					SysLogToFile(fmt.Sprintf("[时间同步] 检测间隔已调整为 %d 秒", interval))
				}
				checkTimeOnce(latest)
			}
		}()
	})
}
