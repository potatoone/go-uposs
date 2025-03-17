
## 1.项目介绍

GO-UPOSS 是一个基于 Go 语言开发的桌面应用程序，主要用于处理图片上传至 Minio 对象存储服务的工作流程。该应用提供图形用户界面（GUI），并支持系统托盘功能、自动任务执行、定时任务调度等功能。

### **主要功能**


1. **图片处理与上传**
   * 从指定源路径复制图片到本地文件夹
   * 支持图片压缩和大小调整
   * 将处理后的图片上传到 Minio 对象存储服务
2. **API 集成**
   * 根据文件名查询 API1
   * 返回下载链接推送至 API2
3. **任务调度与自动化**
   * 支持手动触发任务
   * 支持定时任务执行
   * 可配置程序启动时自动执行任务
4. **系统集成**
   * 支持开机自启动
   * 提供系统托盘功能，最小化运行
   * 界面锁定功能，通过硬编码密码防止误操作
5. **数据管理**
   * 基于 SQLite 的文件增量复制检索
   * 按日期轮转的任务日志
   * 提供定期清理日志和数据库记录的功能
6. **部署与分发**
   * 使用 Fyne 的打包工具生成独立可执行文件
   * 支持 Windows 平台，提供图标和应用程序标识

### **技术栈**


1. **Go 语言**：核心编程语言
2. **Fyne**：GUI 框架，提供跨平台界面支持
3. **Minio SDK**：与 Minio 对象存储服务交互
4. **SQLite**：本地数据存储
5. **Windows API**：系统集成（如开机自启动）

   \

## **2.功能模块详解**

### **配置管理**

* 通过 JSON 文件存储配置
* 支持多种配置项，如 Minio 连接信息、文件路径、图片处理参数等
* 提供图形界面修改配置

### **图片处理流程**


1. 从源路径获取图片
2. 根据配置参数进行压缩和大小调整
3. 保存到本地临时文件夹
4. 上传到 Minio 服务器
5. 通过 API 集成实现后续业务流程

### **任务调度**

* 自动任务：可配置间隔时间自动执行
* 计划任务：按指定时间表执行
* 支持手动触发和程序启动时自动执行

### **界面安全**

* 支持界面锁定功能，防止未授权访问
* 通过硬编码密码验证（实际应用中应考虑更安全的方式）

### **数据清理**

* 定期清理日志文件，避免占用过多磁盘空间
* 清理数据库中的历史记录，优化性能


## **3.项目结构**

go-uposs/

├── main.go                     # 主程序入口

├── config.go                  # 配置文件处理

├── auto.go                     # 自动任务相关功能

├── sched.go                   # 计划任务相关功能

├── match_copy.go         # 文件夹&文件复制

├── clean.go                    # 清理日志&数据库功能

├── minio_client.go         # Minio客户端

├── oss_config.go           # OSS配置

├── folder_config.go       # 文件夹配置

├── about.go                   # 关于页面和其他设置

├── logger.go                  # 日志记录功能

├── date.go                     # 日期UI组件

├── pic_handle.go           # 图片压缩功能

├── utils/                          # 工具函数

│   ├── passwd.go          # 密码验证功能

│   ├── path.go               # 路径处理功能

│   ├── port.go                # 端口监听

│   ├── divide.go             # 文件名切割

│   └── autostart.go        # 开机自启动功能

├── database/                 # 数据库相关

│   ├── queries.go           # 数据库查询

│   └── database.go       # 数据库初始化

├── data/                         # 资源文件

│   ├── icon/                    # 图标文件

│   └── config.example.json    # 配置文件

├── go.mod                     # 依赖管理

└── go.sum                     # 依赖版本锁定


## 4.开发环境配置

### 安装 Windows GO 语言

<https://golang.google.cn/dl/>

<https://golang.google.cn/dl/go1.24.1.windows-amd64.msi>

### 安装 Windows C 编译器 

csdn安装参考 <https://blog.csdn.net/iamhuanyu/article/details/129670053>

<https://www.msys2.org/> 

### 安装依赖

初始化 Go 模块 `go mod init go-uposs`

安装 go minio sdk `go get github.com/minio/minio-go/v7`

清理和重装依赖 `go mod tidy` `go mod vendor`

安装 sqlite3 `go get -u github.com/mattn/go-sqlite3`

### 配置Windows GUI 

安装 Fyne 库 `go get fyne.io/fyne/v2` `go get fyne.io/fyne/v2/dialog`

### 打包EXE

安装Fyne的命令行工具 

`go install fyne.io/fyne/v2/cmd/fyne@latest`

使用 -name 参数自定义生成的 exe 

`fyne package -os windows -icon .\data\icon\icon.ico -name "GO-UPOSS"`

打包 Release 程序到根目录 

`fyne package -os windows -icon .\data\icon\icon.ico -name "GO-UPOSS" -appID "com.apotato.go-uposs" -release`