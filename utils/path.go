package utils

import (
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	// DocumentsPath 存储 Windows 用户的文档文件夹路径
	DocumentsPath string

	// GoupossPath 存储 gouposs 应用程序在用户文档目录中的路径
	GoupossPath string

	// DataPath 存储 gouposs 应用程序数据文件夹路径
	DataPath string

	// AutoLogPath 存储 gouposs 应用程序自动复制日志文件夹路径
	AutoLogPath string

	// SchedLogPath 存储 gouposs 应用程序计划复制日志文件夹路径
	SchedLogPath string
)

// 初始化模块变量
func init() {
	DocumentsPath = GetWindowsDocumentsPath()
	GoupossPath = filepath.Join(DocumentsPath, "gouposs")  // 硬编码程序文件夹路径
	DataPath = filepath.Join(GoupossPath, "data")          // 构建 data 文件夹路径
	AutoLogPath = filepath.Join(GoupossPath, "log_auto")   // 构建 auto log 文件夹路径
	SchedLogPath = filepath.Join(GoupossPath, "log_sched") // 构建 sched log 文件夹路径

	// 确保所有目录存在
	EnsureDirExists(DataPath)
	EnsureDirExists(AutoLogPath)
	EnsureDirExists(SchedLogPath)

	log.Printf("DocumentsPath: %s\n", DocumentsPath)
	log.Printf("GoupossPath: %s\n", GoupossPath)
	log.Printf("DataPath: %s\n", DataPath)
	log.Printf("AutoLogPath: %s\n", AutoLogPath)
	log.Printf("SchedLogPath: %s\n", SchedLogPath)
}

// GetWindowsDocumentsPath 获取当前 Windows 用户的 Documents 文件夹路径
func GetWindowsDocumentsPath() string {
	// 首先尝试使用环境变量方式获取
	docPath := os.Getenv("USERPROFILE")
	if docPath != "" {
		docPath = filepath.Join(docPath, "Documents")
		if _, err := os.Stat(docPath); err == nil {
			return docPath
		}
	}

	// 如果环境变量方式失败，使用 Windows API 获取
	return getDocumentsPathUsingAPI()
}

// getDocumentsPathUsingAPI 使用 Windows API 获取 Documents 路径
func getDocumentsPathUsingAPI() string {
	// 加载 shell32.dll
	shell32 := syscall.NewLazyDLL("shell32.dll")

	// 获取 SHGetFolderPath 函数
	shGetFolderPath := shell32.NewProc("SHGetFolderPathW")

	// CSIDL for My Documents
	const CSIDL_PERSONAL = 0x0005

	// 为路径分配缓冲区
	buf := make([]uint16, syscall.MAX_PATH)

	// 调用 Windows API 获取 Documents 文件夹路径
	ret, _, _ := shGetFolderPath.Call(
		0,                                // hwndOwner [in, optional]
		uintptr(CSIDL_PERSONAL),          // nFolder [in]
		0,                                // hToken [in, optional]
		0,                                // dwFlags [in]
		uintptr(unsafe.Pointer(&buf[0])), // pszPath [out]
	)

	// 检查返回值
	if ret != 0 {
		// 如果 API 调用失败，返回默认路径
		home := os.Getenv("USERPROFILE")
		if home == "" {
			home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		}
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Documents")
	}

	// 将 UTF-16 编码的路径转换为字符串
	return syscall.UTF16ToString(buf)
}

// IsPathExists 检查路径是否存在
func IsPathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// EnsureDirExists 确保目录存在，如果不存在则创建
func EnsureDirExists(path string) error {
	if !IsPathExists(path) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return nil
}
