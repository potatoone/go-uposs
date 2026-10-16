package main

import (
	"fmt"
	"go-uposs/internal/utils"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/nfnt/resize"
)

// isSupportedImage 判断文件扩展名是否为支持的图片格式
func isSupportedImage(fileName string) bool {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".jpeg", ".jpg", ".png", ".gif":
		return true
	default:
		return false
	}
}

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

// HandleImages 处理 local_folder 下的所有图像文件，按 concurrency 限制同时处理的文件数量
func HandleImages(folder, compress, width string, picSize int, concurrency int, isScheduledTask bool) error {
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

	limiter := newFileConcurrencyLimiter(concurrency)
	var wg sync.WaitGroup

	err = filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 只处理大于等于 picSize KB 的 JPEG、PNG 和 GIF 文件
		if !info.IsDir() && info.Size() >= int64(picSize*1024) && isSupportedImage(info.Name()) {

			// 记录开始处理
			logMsg := fmt.Sprintf("正在处理文件: %s", path)
			if isScheduledTask {
				SchedLogToFile(logMsg)
			} else {
				AutoLogToFile(logMsg)
			}

			wg.Add(1)
			limiter.acquire() // 占用一个文件并发名额
			go func(path string) {
				defer wg.Done()
				defer limiter.release()

				// 处理图片，如果失败则记录错误并继续
				if err := CompressImage(path, quality, widthInt); err != nil {
					errMsg := fmt.Sprintf("处理文件 %s 失败: %v", path, err)
					if isScheduledTask {
						SchedLogToFile(errMsg)
					} else {
						AutoLogToFile(errMsg)
					}
					return
				}

				// 记录处理完成
				successMsg := fmt.Sprintf("文件处理完成: %s", path)
				if isScheduledTask {
					SchedLogToFile(successMsg)
				} else {
					AutoLogToFile(successMsg)
				}
			}(path)
		}

		return nil
	})

	// 等待所有已启动的压缩任务结束
	wg.Wait()

	return err
}
