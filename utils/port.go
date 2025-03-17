package utils

import (
	"log"
	"net"
	"net/http"
	"strconv"
)

// 全局常量定义
const (
	// 应用程序使用的固定端口号
	AppPort = 9999 // 选一个不太常用的端口
)

// 检查程序是否已经运行
func ListenPort() bool {
	// 尝试监听指定端口
	listener, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(AppPort))
	if err != nil {
		// 端口被占用，程序可能已经在运行
		log.Println("无法绑定到端口", AppPort, "，可能被其他程序占用")
		return true
	}
	// 保持端口监听
	go func() {
		defer listener.Close()

		// 设置简单的HTTP服务器，用于健康检查和状态查询
		http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"running", "app":"go-uposs"}`))
		})

		// 启动HTTP服务
		log.Println("程序启动，监听端口:", AppPort)
		if err := http.Serve(listener, nil); err != nil {
			log.Println("HTTP服务器错误:", err)
		}
	}()

	return false
}
