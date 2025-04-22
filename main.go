package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sys/windows/registry"

	"go-uposs/utils"
)

// MainLogToFile 将主程序日志写入系统日志
func MainLogToFile(message string) {
	// 只添加标识符，不添加时间戳
	formattedMessage := fmt.Sprintf("[MAIN] %s", message)

	// 使用系统日志函数记录
	SysLogToFile(formattedMessage)
}

// 确保初始化数据库
func initDatabase() error {
	MainLogToFile("初始化数据库")

	// 使用 utils.DataPath 作为数据目录
	dataDir := utils.DataPath

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		MainLogToFile(fmt.Sprintf("创建数据目录失败: %v", err))
		return fmt.Errorf("创建数据目录失败: %v", err)
	}

	dbPath := filepath.Join(dataDir, "uposs.db")
	dbConfig := &utils.DBConfig{
		DBPath: dbPath,
	}

	return utils.InitDB(dbConfig)
}

// ShowPasswordDialogIfNeeded 根据配置决定是否显示密码对话框
func ShowPasswordDialogIfNeeded(window fyne.Window, config *Config) {
	// 检查是否启用了界面锁定
	if config == nil || config.LockUI != "true" {
		// 如果未启用，直接显示窗口
		window.Show()
		return
	}

	// 显示密码对话框，获取验证完成通知通道
	passwordDone := utils.ShowPasswordDialogSync(window)

	// 启动异步等待密码验证
	go func() {
		<-passwordDone
		// 密码验证成功时对话框会关闭，主窗口会自动变为可见
		MainLogToFile("密码验证成功，窗口已显示")
	}()

	// 标记窗口为"应该显示"状态，实际上窗口会在密码对话框关闭后才会显示
	window.Show()
}

func main() {

	// 绑定受监听端口
	utils.ListenPort()

	// 初始化数据库
	if err := initDatabase(); err != nil {
		MainLogToFile(fmt.Sprintf("初始化数据库失败: %v", err))
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer utils.CloseDB()

	// 加载配置
	config, err := LoadConfig("config.json") // 使用相对路径
	if err != nil {
		MainLogToFile(fmt.Sprintf("初始化配置失败: %v", err))
		log.Fatalf("初始化配置失败: %v", err)
	}

	// 同步开机自启动配置
	if config.AutoStart == "true" {
		if err := utils.EnableAutoStart(); err != nil {
			MainLogToFile(fmt.Sprintf("启用开机自启动失败: %v", err))
		}
	} else {
		if err := utils.DisableAutoStart(); err != nil {
			MainLogToFile(fmt.Sprintf("禁用开机自启动失败: %v", err))
		}
	}

	// 创建 Fyne 应用
	myApp := app.NewWithID("com.apotato.gouposs")
	myWindow := myApp.NewWindow("IMG Upload To Minio")

	// 设置自定义主题
	customTheme, _ := utils.NewCustomTheme()
	myApp.Settings().SetTheme(customTheme)

	// 创建自动执行 UI
	autoTaskUI := createautoTaskUI(myWindow, config)

	// 创建计划执行 UI
	schedUI := createSchedUI(config, myWindow)

	// 创建文件夹配置 UI
	folderConfigUI := createFolderConfigUI(config, myWindow)

	// 创建配置 UI
	configUI := CreateUI(config, myWindow)

	// 创建图片配置 UI
	picConfigUI := createPicConfigUI(config, myWindow)

	// 创建关于界面 UI
	aboutUI := createAboutUI(myWindow)

	// 创建api配置界面 UI
	apiconfigUI := createAPIConfigUI(config, myWindow)

	// 创建 Tab 内容，并添加内边距
	autotaskTab := container.NewTabItem("自动任务", container.NewVBox(container.NewPadded(autoTaskUI)))
	schedTab := container.NewTabItem("计划任务", container.NewVBox(container.NewPadded(schedUI)))
	folderConfigTab := container.NewTabItem("文件夹配置", container.NewVBox(container.NewPadded(folderConfigUI)))
	configUITab := container.NewTabItem("OSS 配置", container.NewVBox(container.NewPadded(configUI)))
	picConfigTab := container.NewTabItem("图片配置", container.NewVBox(container.NewPadded(picConfigUI)))
	apiConfigTab := container.NewTabItem("API配置", container.NewVBox(container.NewPadded(apiconfigUI)))
	aboutTab := container.NewTabItem("关于", container.NewVBox(container.NewPadded(aboutUI)))

	// 创建 Tabs
	tabs := container.NewAppTabs(
		autotaskTab,
		schedTab,
		folderConfigTab,
		configUITab,
		picConfigTab,
		apiConfigTab,
		aboutTab,
	)

	// 设置 Tab 显示顺序和默认选择的标签
	tabs.SetTabLocation(container.TabLocationTop)
	tabs.Select(autotaskTab)

	// 设置窗口内容为 Tab 组件
	myWindow.SetContent(tabs)

	// 在 main 函数内适当位置
	// 添加系统托盘功能
	if desk, ok := myApp.(desktop.App); ok {
		MainLogToFile("初始化系统托盘")
		menu := fyne.NewMenu("GO-UPOSS",
			fyne.NewMenuItem("打开", func() {
				ShowPasswordDialogIfNeeded(myWindow, config)
			}),
		)
		desk.SetSystemTrayMenu(menu)
	}

	// 处理自动执行任务的逻辑
	shouldAutoRunTask := config.AutoStartAutoTask == "true"

	// 修改注册表启动命令
	if utils.IsAutoStartEnabled() {
		exePath, _ := os.Executable()
		key, _ := registry.OpenKey(registry.CURRENT_USER, utils.RegistryAutoRunPath, registry.SET_VALUE)
		key.SetStringValue(utils.AppRegKeyName, fmt.Sprintf(`"%s"`, exePath)) // 移除 --minimized 参数
		key.Close()
	}

	// 程序启动行为设置 - 修改为默认最小化
	myApp.Lifecycle().SetOnStarted(func() {
		// 确保窗口内容已设置
		if myWindow.Content() == nil {
			MainLogToFile("警告：窗口内容为空")
		}

		// 默认最小化
		myWindow.Hide()
		MainLogToFile("程序启动时已最小化到系统托盘")

		// 检查是否自动执行任务
		if shouldAutoRunTask {
			MainLogToFile("触发自动任务执行...")
			go triggerAutoTask()
		}
	})

	// 设置窗口关闭行为为最小化到托盘
	myWindow.SetCloseIntercept(func() {

		// 创建自定义对话框，包含复选框
		exitCheck := widget.NewCheck("直接退出程序", nil)

		// 创建对话框内容
		content := container.NewVBox(
			exitCheck,
		)

		// 创建带有确定和取消按钮的自定义对话框
		confirmDialog := dialog.NewCustomConfirm(
			"是否转到后台运行",
			"确定",
			"取消",
			content,
			func(confirmed bool) {
				if !confirmed {
					return
				}

				if exitCheck.Checked {
					myApp.Quit()
				} else {
					// 否则最小化到托盘
					myWindow.Hide()
					MainLogToFile("窗口已最小化到系统托盘")

					// 手动关闭时最小化磁铁通知
					notification := fyne.NewNotification("GO-UPOSS 已最小化", "程序将在后台继续运行，点击系统托盘图标可重新打开界面。")
					myApp.SendNotification(notification)
				}
			},
			myWindow,
		)

		confirmDialog.Show()
	})

	myApp.Run()
}
