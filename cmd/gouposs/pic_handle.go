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

	xdraw "golang.org/x/image/draw"
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
// 内存要点：解码后的像素缓冲按「宽 × 高 × 字节/像素」占用，与文件大小无关，
// 而重采样又需要一份与原图等大的临时缓冲。这里改用单遍缩放（x/image/draw），
// 相比原先的 Lanczos3（内部两遍 + 转 RGBA）可显著降低单张峰值占用。
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

	// 调整图像大小（单遍缩放，避免额外的全尺寸中间缓冲）
	newImg := scaleImage(img, width)
	img = nil // 及时释放原图，减少同时存活的大对象

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

// scaleImage 按目标宽度等比缩放图像，宽度已满足时直接返回原图
//
// 实现要点（内存与质量兼顾）：
// 核函数类缩放（Lanczos/CatmullRom/BiLinear）需要一份
// 「目标宽度 × 源高度 × 32 字节」的浮点中间缓冲，4000×3000 缩到 1000 宽时约 95 MB；
// 这里改为「逐级减半 + 双线性」：每步恰好缩一半，等价于面积平均（大比例缩小时质量最好），
// 且中间图逐级变小，实测 4000×3000 单张分配由约 95 MB 降到约 14 MB。
func scaleImage(img image.Image, targetWidth int) image.Image {
	bounds := img.Bounds()
	if targetWidth <= 0 || bounds.Dx() <= targetWidth {
		return img
	}

	// 逐级减半，直到再减半就会小于目标宽度
	current := img
	for current.Bounds().Dx()/2 >= targetWidth {
		cb := current.Bounds()
		half := image.NewRGBA(image.Rect(0, 0, cb.Dx()/2, cb.Dy()/2))
		xdraw.ApproxBiLinear.Scale(half, half.Bounds(), current, cb, xdraw.Src, nil)
		current = half
	}

	cb := current.Bounds()
	// 恰好等于目标宽度时直接使用，省掉一次拷贝
	if cb.Dx() == targetWidth {
		return current
	}

	// 最后一步的缩放比例小于 2，双线性即可保证质量
	targetHeight := int(float64(cb.Dy()) * float64(targetWidth) / float64(cb.Dx()))
	if targetHeight < 1 {
		targetHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), current, cb, xdraw.Src, nil)
	return dst
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

				// 图片处理会产生大量临时大对象，堆占用偏高时把空闲内存还给系统
				ReleaseMemoryIfNeeded()
			}(path)
		}

		return nil
	})

	// 等待所有已启动的压缩任务结束
	wg.Wait()

	return err
}
