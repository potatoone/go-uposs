package utils

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	// 硬编码密码，实际应用中应考虑更安全的方式存储
	hardcodedPassword = "1234"
)

// IsLockUIEnabled 检查是否启用了界面锁定
func IsLockUIEnabled(lockUIValue string) bool {
	return lockUIValue == "true"
}

// ShowPasswordDialogSync 显示密码输入对话框
func ShowPasswordDialogSync(window fyne.Window) chan struct{} {
	done := make(chan struct{})

	// 创建密码输入框
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("请输入密码")

	// 创建状态标签，用于显示错误信息
	statusLabel := widget.NewLabel("")
	statusLabel.Hide()

	// 创建变量引用对话框
	var d dialog.Dialog

	// 处理尝试登录的逻辑
	tryLogin := func() {
		if ValidatePassword(passwordEntry.Text) {
			// 密码正确时，关闭通道并显示窗口
			close(done)

			// 关闭对话框
			if d != nil {
				d.Hide()
			}

			// 确保窗口可见
			window.Show()
			window.RequestFocus()
		} else {
			// 密码错误，显示错误信息，并保持窗口隐藏
			statusLabel.SetText("密码错误，请重试")
			statusLabel.Show()

			// 确保窗口保持隐藏状态
			window.Hide()
		}
	}

	// 创建自定义确认对话框
	d = dialog.NewCustomConfirm(
		"界面锁定",
		"进入",
		"最小化",
		container.NewVBox(
			widget.NewLabel("请输入密码以继续"),
			passwordEntry,
			statusLabel,
		),
		func(confirmed bool) {
			if confirmed {
				// 用户点击"进入"按钮
				tryLogin()
			} else {
				// 用户点击"最小化"按钮 - 保持窗口隐藏
				window.Hide()
			}
		},
		window,
	)

	// 设置输入框回车键处理
	passwordEntry.OnSubmitted = func(string) {
		tryLogin()
	}

	// 设置对话框关闭处理（如按ESC键）
	d.SetOnClosed(func() {
		// 如果对话框被关闭但密码未验证成功，确保窗口保持隐藏
		select {
		case <-done:
			// 密码已验证成功，不需要操作
		default:
			// 密码未验证成功，确保窗口隐藏
			window.Hide()
		}
	})

	// 显示对话框前，确保窗口处于隐藏状态
	window.Hide()

	// 显示对话框
	d.Show()

	// 设置焦点到密码输入框
	window.Canvas().Focus(passwordEntry)

	return done
}

// ValidatePassword 验证密码是否正确
func ValidatePassword(password string) bool {
	return password == hardcodedPassword
}
