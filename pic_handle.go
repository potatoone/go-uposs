package main

import (
	"fmt"
	"go-uposs/utils"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nfnt/resize"
)

// CompressImage 根据配置压缩图像
// CompressImage 根据配置压缩图像
func CompressImage(srcPath string, quality, width int) error {
	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("无法打开源文件: %v", err)
	}

	// 解码图像
	var img image.Image
	switch {
	case strings.HasSuffix(strings.ToLower(srcPath), ".jpeg"), strings.HasSuffix(strings.ToLower(srcPath), ".jpg"):
		img, err = jpeg.Decode(srcFile)
	case strings.HasSuffix(strings.ToLower(srcPath), ".png"):
		img, err = png.Decode(srcFile)
	case strings.HasSuffix(strings.ToLower(srcPath), ".gif"):
		img, err = gif.Decode(srcFile)
	}

	// 关闭文件，释放文件锁
	srcFile.Close()

	if err != nil {
		// 尝试删除数据库记录
		fileName := filepath.Base(srcPath)
		if delDBErr := utils.DeleteFileCopyRecord(fileName, true); delDBErr != nil {
			return fmt.Errorf("\n解码图像失败、删除数据库记录失败且删除文件失败: 解码错误 %v, 删除数据库记录错误 %v", err, delDBErr)
		}
		// 尝试删除无法解码的图片
		if delErr := os.Remove(srcPath); delErr != nil {
			return fmt.Errorf("\n解码图像失败、删除数据库记录成功但删除文件失败: 解码错误 %v, 删除文件错误 %v", err, delErr)
		}
		return fmt.Errorf("\n解码图像失败，已删除数据库记录和文件: %v", err)
	}

	// 调整图像大小
	newImg := resize.Resize(uint(width), 0, img, resize.Lanczos3)

	// 创建目标文件（覆盖源文件）
	destFile, err := os.Create(srcPath) // 使用源文件路径覆盖原文件
	if err != nil {
		return fmt.Errorf("无法创建目标文件: %v", err)
	}
	defer destFile.Close()

	// 压缩图像并保存为新文件
	switch {
	case strings.HasSuffix(strings.ToLower(srcPath), ".jpeg"), strings.HasSuffix(strings.ToLower(srcPath), ".jpg"):
		opts := jpeg.Options{Quality: quality}
		err = jpeg.Encode(destFile, newImg, &opts)
	case strings.HasSuffix(strings.ToLower(srcPath), ".png"):
		err = png.Encode(destFile, newImg)
	case strings.HasSuffix(strings.ToLower(srcPath), ".gif"):
		err = gif.Encode(destFile, newImg, nil)
	}
	if err != nil {
		return fmt.Errorf("压缩图像失败: %v", err)
	}

	return nil
}

// HandleImages 处理 local_folder 下的所有图像文件
func HandleImages(folder, compress, width string, picSize int, isScheduledTask bool) error {
	quality, err := strconv.Atoi(compress)
	if err != nil {
		return fmt.Errorf("压缩比率转换失败: %v", err)
	}

	if quality < 0 || quality > 100 {
		return fmt.Errorf("压缩比率应在 0 到 100 之间")
	}

	widthInt, err := strconv.Atoi(width)
	if err != nil {
		return fmt.Errorf("宽度配置转换失败: %v", err)
	}

	return filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理大于等于 picSize KB 的 JPEG、PNG 和 GIF 文件
		if !info.IsDir() && info.Size() >= int64(picSize*1024) && (strings.HasSuffix(strings.ToLower(info.Name()), ".jpeg") || strings.HasSuffix(strings.ToLower(info.Name()), ".jpg") || strings.HasSuffix(strings.ToLower(info.Name()), ".png") || strings.HasSuffix(strings.ToLower(info.Name()), ".gif")) {
			// 根据任务类型选择不同的日志记录函数
			if isScheduledTask {
				SchedLogToFile(fmt.Sprintf("正在处理文件: %s", path))
			} else {
				AutoLogToFile(fmt.Sprintf("正在处理文件: %s", path))
			}

			err := CompressImage(path, quality, widthInt)
			if err != nil {
				return fmt.Errorf("处理文件 %s 失败: %v", path, err)
			}

			// 根据任务类型选择不同的日志记录函数
			if isScheduledTask {
				SchedLogToFile(fmt.Sprintf("文件处理完成: %s", path))
			} else {
				AutoLogToFile(fmt.Sprintf("文件处理完成: %s", path))
			}
		}

		return nil
	})
}
