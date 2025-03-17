package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	RegistryAutoRunPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run` // 导出常量
	AppRegKeyName       = "GO-UPOSS"                                      // 导出常量
)

// IsAutoStartEnabled 检查程序是否已设置为开机自启动
func IsAutoStartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, RegistryAutoRunPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	val, _, err := key.GetStringValue(AppRegKeyName)
	if err != nil {
		return false
	}

	// 检查注册表中的路径是否与当前程序路径一致
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	// 规范化路径以便比较
	exePath = strings.ReplaceAll(strings.ToLower(exePath), "/", "\\")
	val = strings.ReplaceAll(strings.ToLower(val), "/", "\\")

	// 比较路径（忽略引号差异）
	return strings.TrimSpace(strings.Trim(val, `"`)) == strings.TrimSpace(exePath)
}

// EnableAutoStart 设置程序开机自启动
func EnableAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, RegistryAutoRunPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("无法访问注册表: %v", err)
	}
	defer key.Close()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取程序路径失败: %v", err)
	}

	// 确保使用绝对路径且带引号（处理路径中的空格）
	exePath = filepath.Clean(exePath)

	// 设置注册表项
	err = key.SetStringValue(AppRegKeyName, fmt.Sprintf(`"%s"`, exePath))
	if err != nil {
		return fmt.Errorf("设置注册表值失败: %v", err)
	}

	return nil
}

// DisableAutoStart 取消程序开机自启动
func DisableAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, RegistryAutoRunPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("无法访问注册表: %v", err)
	}
	defer key.Close()

	// 删除注册表项
	err = key.DeleteValue(AppRegKeyName)
	if err != nil {
		// 如果键不存在，不视为错误
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("删除注册表值失败: %v", err)
	}

	return nil
}
