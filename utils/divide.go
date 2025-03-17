package utils

import (
	"path/filepath"
	"regexp"
	"strings"
)

// ParseImageName 解析图片名称，通过识别逗号和横杠对文件名中的单号进行分隔识别，排除掉括号包围的字符串
func ParseImageName(fileName string) []string {
	// 去掉文件扩展名
	fileName = strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// 去掉文件名中的括号包围的字符串
	re := regexp.MustCompile(`\([^)]*\)`)
	cleanedFileName := re.ReplaceAllString(fileName, "")

	// 使用逗号和横杠进行分隔
	parts := regexp.MustCompile(`[,-]`).Split(cleanedFileName, -1)

	// 去掉前后空格并排除空字符串
	var result []string

	// 编译正则表达式
	reSingleNumber := regexp.MustCompile(`^[a-zA-Z0-9]{3,}$`)

	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart != "" {
			// 只保留可能是单号的部分（例如：字母和数字组合）
			// 这里假设单号由字母和数字组成，长度至少为3
			if reSingleNumber.MatchString(trimmedPart) {
				result = append(result, trimmedPart)
			}
		}
	}

	return result
}
