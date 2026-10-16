# GO-UPOSS
![image](https://github.com/potatoone/go-uposs/blob/main/data/screenshot/1.png)

## 1.项目介绍

GO-UPOSS 是一个基于 Go 语言开发的桌面应用程序，主要用于处理图片上传至 Minio 对象存储服务的工作流程。该应用提供图形用户界面（GUI），并支持系统托盘功能、自动任务执行、定时任务调度等功能，配置API1查询和API2推送接口，将文件名匹配系统特定编号携带OSS链接推送（接口请求和推送方式固定，需自行修改实现）。同时内置系统监控功能，实时采集主机 CPU、内存、磁盘信息，并通过 UDP 心跳上报服务器。

API2-test 测试用例，本地运行 python api2_test_server.py 
API2地址：http://localhost:3001

### **源码编译运行**
请在当前登录用户的 Documents 目录创建 gouposs/data 目录，将 config.example.json 修改为 config.json 存放 data 下，minio配置为官方测试实例，切勿传输隐私数据，误操作自行删除桶

### **Release**
   * 使用 portable 便携版，释放文件夹在 Documents
   * 使用 installer 安装版本可直接运行


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
7. **系统监控（心跳）**
   * 实时采集主机 CPU 使用率、内存、磁盘信息
   * 程序启动即自动发送 UDP 心跳（无需回包）
   * 心跳地址和间隔可配置，间隔默认 60 秒
   * 心跳日志仅在软件界面显示（最多 10 条），不写入文件日志
   * 手动停止后，下次重新启动仍会自动发送心跳

### **技术栈**


1. **Go 语言**：核心编程语言
2. **Fyne**：GUI 框架，提供跨平台界面支持
3. **Minio SDK**：与 Minio 对象存储服务交互
4. **SQLite**：本地数据存储
5. **Windows API**：系统集成（如开机自启动）


## **2.功能模块详解**

### **配置管理**

* 通过 JSON 文件存储配置
* 支持多种配置项，如 Minio 连接信息、文件路径、图片处理参数等
* 提供图形界面修改配置

### **性能相关配置（重要）**

| 配置项 | 单位 | 默认 | 取值范围 | 说明 |
|---|---|---|---|---|
| `io_buffer` | **KB** | 1024 | 4 ~ 8192 | 复制文件时的缓冲区大小。**每个并发复制协程各占一块**，所以总占用约为「该值 × 复制并发数」 |
| `copy_concurrency` | 个 | 1 | 1 ~ 32 | 同时复制的文件数 |
| `compress_concurrency` | 个 | 1 | 1 ~ 32 | 同时压缩的文件数 |
| `upload_concurrency` | 个 | 1 | 1 ~ 32 | 同时上传的文件数 |
| `cpu_threads` | 线程 | 0（=全部核心） | 1 ~ 逻辑核心数 | 程序可占用的 CPU 线程数 |

内存估算示例：`io_buffer = 1024`（1 MB）、`copy_concurrency = 20` → 复制阶段最多约 20 MB 缓冲。
若把 `io_buffer` 填成 409600（= 400 MB），在 20 并发下会申请约 8 GB 内存，因此程序已把该值限制在 8 MB 以内。

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

### **系统监控**

* 通过 Windows API（GetSystemTimes、GlobalMemoryStatusEx、GetDiskFreeSpaceExW）采集 CPU、内存、磁盘信息
* 程序启动即自动启动 UDP 心跳，无需回包，fire-and-forget 模式
* 心跳数据以 JSON 格式发送，包含机器码、时间戳、CPU、内存、磁盘信息
* 心跳地址支持自动补全协议前缀（本地地址自动使用 http://）


## **3.项目结构**

go-uposs/

├── cmd/

│   └── gouposs/                # 主程序（main 包，程序入口 main.go 与全部业务代码）

│       ├── main.go             # 主程序入口：初始化、窗口、Tab 组装

│       ├── config.go           # 配置结构体与读写、并发/时间同步默认值

│       ├── config_api.go       # API 配置界面

│       ├── config_oss.go       # OSS 配置界面

│       ├── config_folder.go    # 文件夹配置界面

│       ├── config_pic.go       # 图片配置界面

│       ├── task_auto.go        # 自动任务

│       ├── task_sched.go       # 计划任务

│       ├── tasklock.go         # 全局任务锁与可中断等待

│       ├── match.go            # 日期/文件夹匹配

│       ├── match_copy.go       # 文件夹与文件复制（支持并发）

│       ├── pic_handle.go       # 图片压缩（支持并发）

│       ├── upload.go           # 上传到 MinIO + API1/API2 流程（支持并发）

│       ├── minio_client.go     # MinIO 客户端

│       ├── timesync.go         # 网络时间检测与偏差告警

│       ├── webhook.go          # 企业微信通知（含发送冷却）

│       ├── notify.go           # 通知冷却状态

│       ├── monitor.go          # 系统监控与心跳

│       ├── logger.go           # 日志记录

│       ├── about.go            # 关于页面与其他设置

│       ├── clean.go            # 清理日志与数据库记录

│       ├── date.go             # 日期选择 UI 组件

│       └── FyneApp.toml        # Fyne 应用元数据（图标、名称、版本）

├── internal/

│   └── utils/                  # 内部工具包（仅本项目可导入）

│       ├── passwd.go           # 密码验证

│       ├── path.go             # 路径处理

│       ├── port.go             # 端口监听

│       ├── divide.go           # 文件名切割

│       ├── sysinfo.go          # 系统信息采集、心跳发送、进程资源占用

│       ├── database.go         # 数据库初始化与复制记录读写

│       ├── api.go              # API1 查询 / API2 推送

│       ├── autostart.go        # 开机自启动

│       ├── theme.go            # 自定义主题（内嵌字体）

│       └── mk.ttf              # 嵌入字体

├── data/                       # 资源文件

│   ├── icon/                   # 图标（icon.ico / icon.png / 生成脚本）

│   └── config.example.json     # 配置示例

├── project/                    # 安装包工程与历史产物

├── build.ps1                   # 构建/打包脚本

├── go.mod                      # 依赖管理

└── go.sum                      # 依赖版本锁定


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

### 运行调试

`go run ./cmd/gouposs`

### 系统监控配置

配置文件（data/config.json）中的心跳相关字段：

```json
"heartbeat_url": "127.0.0.1:2200",   // 心跳服务器地址（UDP），支持自动补全 http:// 前缀
"heartbeat_interval": "60"            // 心跳间隔（秒），默认 60
```

* 程序启动即自动发送 UDP 心跳（无需回包，fire-and-forget）
* 手动停止后，下一次重新启动依然自动发送心跳
* 心跳日志仅在软件「监控」Tab 界面显示（最多 10 条），不写入文件日志
* 心跳 JSON 格式：`{"machine_code": "...", "timestamp": "...", "cpu": {...}, "memory": {...}, "disks": [...]}`

### 打包EXE

安装Fyne的命令行工具

`go install fyne.io/fyne/v2/cmd/fyne@latest`

一键打包（推荐，自动编译并生成 release 版到仓库根目录）：

`.\build.ps1`

仅校验编译不改动产物：

`.\build.ps1 -NoPackage`

手动打包说明：代码位于 `cmd/gouposs`（main 包），Fyne 要求 `FyneApp.toml` 与 main 包同目录，
因此需先进入该目录再打包，产物会生成在该目录下：

```powershell
cd cmd\gouposs
fyne package -os windows -icon ..\..\data\icon\icon.ico -name "GO-UPOSS" -appID "com.apotato.go-uposs" -release
```

也可在仓库根目录用 `-sourceDir` 指定源码目录：

`fyne package -sourceDir .\cmd\gouposs -os windows -icon .\data\icon\icon.ico -name "GO-UPOSS" -appID "com.apotato.go-uposs" -release`


