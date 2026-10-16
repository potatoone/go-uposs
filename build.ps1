# 构建脚本：编译并打包 GO-UPOSS（Windows release 版）
#
# 用法（在仓库根目录执行）：
#   .\build.ps1                     # 打包 release 版，产物放到仓库根目录
#   .\build.ps1 -NoPackage          # 只做 go vet / go build 校验
#   .\build.ps1 -Icon data\icon\icon.png
#
# 说明：代码结构为 cmd/gouposs（main 包）+ internal/utils。
# fyne package 要求 FyneApp.toml 与 main 包同目录，因此进入 cmd/gouposs 打包。

param(
    [string]$Icon = "data\icon\icon.ico",
    [string]$Name = "GO-UPOSS",
    [string]$AppID = "com.apotato.go-uposs",
    [switch]$NoPackage
)

$ErrorActionPreference = "Stop"
$root = $PSScriptRoot
Set-Location $root

Write-Host "[1/4] go vet ./..." -ForegroundColor Cyan
go vet ./...
if ($LASTEXITCODE -ne 0) { throw "go vet 失败" }

Write-Host "[2/4] go build ./..." -ForegroundColor Cyan
go build ./...
if ($LASTEXITCODE -ne 0) { throw "go build 失败" }

if ($NoPackage) {
    Write-Host "已跳过打包" -ForegroundColor Yellow
    return
}

$iconAbs = (Resolve-Path $Icon).Path

Write-Host "[3/4] fyne package（release）" -ForegroundColor Cyan
Push-Location "cmd\gouposs"
try {
    fyne package -os windows -icon $iconAbs -name $Name -appID $AppID -release
    if ($LASTEXITCODE -ne 0) { throw "fyne package 失败" }
}
finally {
    Pop-Location
}

Write-Host "[4/4] 移动产物到仓库根目录" -ForegroundColor Cyan
$built = Join-Path $root "cmd\gouposs\$Name.exe"
if (-not (Test-Path $built)) { throw "未找到打包产物: $built" }

$target = Join-Path $root "$Name.exe"
try {
    Move-Item $built $target -Force
}
catch {
    Write-Warning "无法覆盖 $target（可能正在运行），产物保留在 $built"
    $target = $built
}

Get-Item $target |
    Select-Object FullName, @{N = "大小(MB)"; E = { [math]::Round($_.Length / 1MB, 2) } }, LastWriteTime |
    Format-List
Write-Host "打包完成" -ForegroundColor Green
