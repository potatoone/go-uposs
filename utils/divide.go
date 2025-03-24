package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// ParseImageName 解析图片名称，通过逗号分割文件名中的单号，排除括号包围的字符串
func ParseImageName(fileName string) []string {
	// 去掉文件扩展名
	fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// 去掉文件名中的括号包围的字符串
	re := regexp.MustCompile(`\([^)]*\)`)
	cleanedFileName := re.ReplaceAllString(fileName, "")

	// 使用逗号进行分割
	parts := regexp.MustCompile(`,`).Split(cleanedFileName, -1)

	// 去掉前后空格并排除空字符串
	var result []string

	// 允许字母、数字和连字符（-），至少3个字符
	reSingleNumber := regexp.MustCompile(`^[a-zA-Z0-9\-]{3,}$`)

	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart != "" {
			if reSingleNumber.MatchString(trimmedPart) {
				result = append(result, trimmedPart)
			}
		}
	}

	return result
}
