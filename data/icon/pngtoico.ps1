function Convert-PngToIco {
    param (
        [string]$PngPath,
        [string]$IcoPath
    )
    # 读取文件
    $png = [System.IO.File]::ReadAllBytes($pngPath)

    # 生成头信息
    $ico = [System.IO.MemoryStream]::new()
    $bin = [System.IO.BinaryWriter]::new($ico)

    # 写入ICONDIR结构
    $bin.Write([uint16]0) # 保留
    $bin.Write([uint16]1) # 图像类型，ico 为 1
    $bin.Write([uint16]1) # 图像数量，1 张

    # 写入ICONDIRENTRY结构
    $bin.Write([sbyte]0) # 宽度
    $bin.Write([sbyte]0) # 高度
    $bin.Write([sbyte]0) # 颜色
    $bin.Write([sbyte]0) # 保留
    $bin.Write([uint16]1) # 颜色平面
    $bin.Write([uint16]32) # 每像素位数（32bpp）
    $bin.Write([uint32]$png.Length) # 图像文件大小
    $bin.Write([uint32]22) # 图像数据偏移量

    # 写入图像数据
    $bin.Write($png)

    [System.IO.File]::WriteAllBytes($icoPath, $ico.ToArray())

    # 清理资源
    $bin.Dispose()
    $ico.Dispose()
}

# 使用示例
$pngPath = ".\icon.png"
$icoPath = ".\icon.ico"
Convert-PngToIco -PngPath $pngPath -IcoPath $icoPath