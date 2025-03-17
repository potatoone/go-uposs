初始化 Go 模块
go mod init go-uposs

安装go minio sdk
go get github.com/minio/minio-go/v7

运行 minio 公共 oss 桶测试
go run main.go

go.sum 依赖版本信息


清理和重装依赖
go mod tidy
go mod vendor

安装sqlite3
go get -u github.com/mattn/go-sqlite3

Go 配置Windows GUI
安装 Fyne 库
go get fyne.io/fyne/v2
go get fyne.io/fyne/v2/dialog

安装Windows推荐的C编译器
https://www.msys2.org/
csdn安装参考
https://blog.csdn.net/iamhuanyu/article/details/129670053


# 打包EXE
安装Fyne的命令行工具
go install fyne.io/fyne/v2/cmd/fyne@latest

使用-name参数自定义生成的exe
fyne package -os windows -icon .\data\icon\icon.png -name "GO-UPOSS"

打包Release程序到根目录
fyne package -os windows -icon .\data\icon\icon.ico -name "GO-UPOSS" -appID "com.apotato.go-uposs" -release

`fyne package -os windows -icon .\data\icon\icon.ico -name "GO-UPOSS" -appID "com.apotato.go-uposs" -release`