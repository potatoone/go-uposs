package utils

import (
	"image/color"

	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// 定义全局常量
const (
	LEBHeight = 33 // 输入框、标签的固定高度 label\entry\button
)

// 定义自定义主题结构体
type CustomTheme struct {
	customFont fyne.Resource
}

// 嵌入字体文件（路径需相对于当前 Go 文件的目录）
//
//go:embed "mk.ttf"
var fontData []byte

// 加载自定义字体文件（无需路径参数）
func NewCustomTheme() (*CustomTheme, error) {
	// 直接使用嵌入的字体数据
	fontResource := fyne.NewStaticResource("mk.ttf", fontData)
	return &CustomTheme{
		customFont: fontResource,
	}, nil
}

// 实现 fyne.Theme 接口的 Font 方法
func (m CustomTheme) Font(style fyne.TextStyle) fyne.Resource {
	return m.customFont // 使用自定义字体
}

// 其他方法保持不变...
func (m CustomTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(n, v)
}

func (m CustomTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (m CustomTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
