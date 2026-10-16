package main

import (
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// GetStartTime 返回配置文件中的开始时间
func GetStartTime(config *Config) string {
	if config.StartTime != "" {
		return config.StartTime
	}
	return time.Now().Format("2006.01.02")
}

// GetEndTime 返回配置文件中的结束时间
func GetEndTime(config *Config) string {
	if config.EndTime != "" {
		return config.EndTime
	}
	return time.Now().Format("2006.01.02")
}

// SetStartTime 更新开始时间并更新 UI 和配置
func SetStartTime(date string, dateLabel *widget.Label, config *Config) {
	dateLabel.SetText(date)
	config.StartTime = date                                  // 只更新开始时间
	config.UpdateTimeRange(config.StartTime, config.EndTime) // 更新配置文件中的时间范围
}

// SetEndTime 更新结束时间并更新 UI 和配置
func SetEndTime(date string, dateLabel *widget.Label, config *Config) {
	dateLabel.SetText(date)
	config.EndTime = date                                    // 只更新结束时间
	config.UpdateTimeRange(config.StartTime, config.EndTime) // 更新配置文件中的时间范围
}

// GetCleanStartTime 返回配置文件中的清理开始时间
func GetCleanStartTime(config *Config) string {
	if config.CleanStartTime != "" {
		return config.CleanStartTime
	}
	return time.Now().AddDate(0, 0, -90).Format("2006.01.02") // 默认为90天前
}

// GetCleanEndTime 返回配置文件中的清理结束时间
func GetCleanEndTime(config *Config) string {
	if config.CleanEndTime != "" {
		return config.CleanEndTime
	}
	return time.Now().AddDate(0, 0, -1).Format("2006.01.02") // 默认为昨天
}

// SetCleanStartTime 更新清理开始时间并更新 UI 和配置
func SetCleanStartTime(date string, dateLabel *widget.Label, config *Config) {
	dateLabel.SetText(date)
	config.CleanStartTime = date                                            // 只更新清理开始时间
	config.UpdateCleanTimeRange(config.CleanStartTime, config.CleanEndTime) // 更新配置文件中的清理时间范围
}

// SetCleanEndTime 更新清理结束时间并更新 UI 和配置
func SetCleanEndTime(date string, dateLabel *widget.Label, config *Config) {
	dateLabel.SetText(date)
	config.CleanEndTime = date                                              // 只更新清理结束时间
	config.UpdateCleanTimeRange(config.CleanStartTime, config.CleanEndTime) // 更新配置文件中的清理时间范围
}

// CreateDateUI 创建日期选择 UI
func CreateDateUI(win fyne.Window, config *Config) fyne.CanvasObject {
	startTime := GetStartTime(config)
	parsedStartTime, _ := time.Parse("2006.01.02", startTime)

	endTime := GetEndTime(config)
	parsedEndTime, _ := time.Parse("2006.01.02", endTime)

	// 定义按钮
	var yearButton, monthButton, dayButton *widget.Button
	widget.NewLabel(time.Now().Format("15:04:05"))
	startTimeLabel := widget.NewLabel(formatDate(parsedStartTime.Year(), int(parsedStartTime.Month()), parsedStartTime.Day())) // 默认显示配置文件中的开始时间
	endTimeLabel := widget.NewLabel(formatDate(parsedEndTime.Year(), int(parsedEndTime.Month()), parsedEndTime.Day()))         // 默认显示配置文件中的结束时间

	// 创建开始时间按钮
	yearButton = widget.NewButton(strconv.Itoa(parsedStartTime.Year()), func() {
		showGridDialog(win, "选择年份", generateYearList(), 5, yearButton, yearButton, monthButton, dayButton, startTimeLabel, config, true)
	})

	monthButton = widget.NewButton(fmt.Sprintf("%02d", parsedStartTime.Month()), func() {
		showGridDialog(win, "选择月份", generateMonthList(), 4, monthButton, yearButton, monthButton, dayButton, startTimeLabel, config, true)
	})

	dayButton = widget.NewButton(fmt.Sprintf("%02d", parsedStartTime.Day()), func() {
		year, _ := strconv.Atoi(yearButton.Text)
		month, _ := strconv.Atoi(monthButton.Text)
		showGridDialog(win, "选择日期", generateDayList(year, month), 6, dayButton, yearButton, monthButton, dayButton, startTimeLabel, config, true)
	})

	// 开始时间选择框
	startDateRow := container.NewHBox(
		yearButton, widget.NewLabel("年"),
		monthButton, widget.NewLabel("月"),
		dayButton, widget.NewLabel("日"),
	)

	// 创建结束时间按钮
	var endYearButton, endMonthButton, endDayButton *widget.Button

	endYearButton = widget.NewButton(strconv.Itoa(parsedEndTime.Year()), func() {
		showGridDialog(win, "选择年份", generateYearList(), 5, endYearButton, endYearButton, endMonthButton, endDayButton, endTimeLabel, config, false)
	})

	endMonthButton = widget.NewButton(fmt.Sprintf("%02d", parsedEndTime.Month()), func() {
		showGridDialog(win, "选择月份", generateMonthList(), 4, endMonthButton, endYearButton, endMonthButton, endDayButton, endTimeLabel, config, false)
	})

	endDayButton = widget.NewButton(fmt.Sprintf("%02d", parsedEndTime.Day()), func() {
		year, _ := strconv.Atoi(endYearButton.Text)
		month, _ := strconv.Atoi(endMonthButton.Text)
		showGridDialog(win, "选择日期", generateDayList(year, month), 6, endDayButton, endYearButton, endMonthButton, endDayButton, endTimeLabel, config, false)
	})

	// 结束时间选择框
	endDateRow := container.NewHBox(
		endYearButton, widget.NewLabel("年"),
		endMonthButton, widget.NewLabel("月"),
		endDayButton, widget.NewLabel("日"),
	)

	// 选择的时间标签
	startInfoRow := container.NewHBox(
		widget.NewLabel("选择开始时间："), startDateRow,
		widget.NewLabel("开始时间："), startTimeLabel,
	)

	endInfoRow := container.NewHBox(
		widget.NewLabel("选择结束时间："), endDateRow,
		widget.NewLabel("结束时间："), endTimeLabel,
	)

	rightContent := container.NewVBox(
		startInfoRow,
		endInfoRow,
	)

	return rightContent
}

// CreateCleanDateUI 创建清理日期选择 UI
func CreateCleanDateUI(win fyne.Window, config *Config) fyne.CanvasObject {
	startTime := GetCleanStartTime(config)
	parsedStartTime, _ := time.Parse("2006.01.02", startTime)

	endTime := GetCleanEndTime(config)
	parsedEndTime, _ := time.Parse("2006.01.02", endTime)

	// 定义按钮
	var yearButton, monthButton, dayButton *widget.Button
	startTimeLabel := widget.NewLabel(formatDate(parsedStartTime.Year(), int(parsedStartTime.Month()), parsedStartTime.Day())) // 默认显示配置文件中的开始时间
	endTimeLabel := widget.NewLabel(formatDate(parsedEndTime.Year(), int(parsedEndTime.Month()), parsedEndTime.Day()))         // 默认显示配置文件中的结束时间

	// 创建开始时间按钮
	yearButton = widget.NewButton(strconv.Itoa(parsedStartTime.Year()), func() {
		showCleanGridDialog(win, "选择年份", generateYearList(), 5, yearButton, yearButton, monthButton, dayButton, startTimeLabel, config, true)
	})

	monthButton = widget.NewButton(fmt.Sprintf("%02d", parsedStartTime.Month()), func() {
		showCleanGridDialog(win, "选择月份", generateMonthList(), 4, monthButton, yearButton, monthButton, dayButton, startTimeLabel, config, true)
	})

	dayButton = widget.NewButton(fmt.Sprintf("%02d", parsedStartTime.Day()), func() {
		year, _ := strconv.Atoi(yearButton.Text)
		month, _ := strconv.Atoi(monthButton.Text)
		showCleanGridDialog(win, "选择日期", generateDayList(year, month), 6, dayButton, yearButton, monthButton, dayButton, startTimeLabel, config, true)
	})

	// 开始时间选择框
	startDateRow := container.NewHBox(
		yearButton, widget.NewLabel("年"),
		monthButton, widget.NewLabel("月"),
		dayButton, widget.NewLabel("日"),
	)

	// 创建结束时间按钮
	var endYearButton, endMonthButton, endDayButton *widget.Button

	endYearButton = widget.NewButton(strconv.Itoa(parsedEndTime.Year()), func() {
		showCleanGridDialog(win, "选择年份", generateYearList(), 5, endYearButton, endYearButton, endMonthButton, endDayButton, endTimeLabel, config, false)
	})

	endMonthButton = widget.NewButton(fmt.Sprintf("%02d", parsedEndTime.Month()), func() {
		showCleanGridDialog(win, "选择月份", generateMonthList(), 4, endMonthButton, endYearButton, endMonthButton, endDayButton, endTimeLabel, config, false)
	})

	endDayButton = widget.NewButton(fmt.Sprintf("%02d", parsedEndTime.Day()), func() {
		year, _ := strconv.Atoi(endYearButton.Text)
		month, _ := strconv.Atoi(endMonthButton.Text)
		showCleanGridDialog(win, "选择日期", generateDayList(year, month), 6, endDayButton, endYearButton, endMonthButton, endDayButton, endTimeLabel, config, false)
	})

	// 结束时间选择框
	endDateRow := container.NewHBox(
		endYearButton, widget.NewLabel("年"),
		endMonthButton, widget.NewLabel("月"),
		endDayButton, widget.NewLabel("日"),
	)

	// 选择的时间标签
	startInfoRow := container.NewHBox(
		widget.NewLabel("选择开始时间："), startDateRow,
		widget.NewLabel("开始时间："), startTimeLabel,
	)

	endInfoRow := container.NewHBox(
		widget.NewLabel("选择结束时间："), endDateRow,
		widget.NewLabel("结束时间："), endTimeLabel,
	)

	return container.NewVBox(
		startInfoRow,
		endInfoRow,
	)
}

// showGridDialog 显示网格选择对话框
func showGridDialog(win fyne.Window, title string, options []string, columns int, targetButton, yearButton, monthButton, dayButton *widget.Button, dateLabel *widget.Label, config *Config, isStartTime bool) {
	var items []fyne.CanvasObject

	for _, option := range options {
		option := option
		button := widget.NewButton(option, func() {
			targetButton.SetText(option)

			year, _ := strconv.Atoi(yearButton.Text)
			month, _ := strconv.Atoi(monthButton.Text)
			day, _ := strconv.Atoi(dayButton.Text)

			// 如果选择的是年份或月份，更新日期按钮的最大值
			if targetButton == yearButton || targetButton == monthButton {
				maxDays := daysInMonth(year, month)
				if day > maxDays {
					day = maxDays
				}
				dayButton.SetText(fmt.Sprintf("%02d", day))
			}

			// 更新选择的日期
			updateSelectedDate(dateLabel, yearButton, monthButton, dayButton)

			// 更新全局 selectedDate 和配置文件中的日期
			if isStartTime {
				SetStartTime(dateLabel.Text, dateLabel, config)
			} else {
				SetEndTime(dateLabel.Text, dateLabel, config)
			}

			// 关闭对话框
			win.Canvas().Overlays().Top().Hide()
		})
		items = append(items, button)
	}

	grid := container.NewGridWithColumns(columns, items...)
	dialog.ShowCustom(title, "关闭", grid, win)
}

// showCleanGridDialog 显示清理日期网格选择对话框
func showCleanGridDialog(win fyne.Window, title string, options []string, columns int, targetButton, yearButton, monthButton, dayButton *widget.Button, dateLabel *widget.Label, config *Config, isStartTime bool) {
	var items []fyne.CanvasObject

	for _, option := range options {
		option := option
		button := widget.NewButton(option, func() {
			targetButton.SetText(option)

			year, _ := strconv.Atoi(yearButton.Text)
			month, _ := strconv.Atoi(monthButton.Text)
			day, _ := strconv.Atoi(dayButton.Text)

			// 如果选择的是年份或月份，更新日期按钮的最大值
			if targetButton == yearButton || targetButton == monthButton {
				maxDays := daysInMonth(year, month)
				if day > maxDays {
					day = maxDays
				}
				dayButton.SetText(fmt.Sprintf("%02d", day))
			}

			// 更新选择的日期
			updateSelectedDate(dateLabel, yearButton, monthButton, dayButton)

			// 更新全局 selectedDate 和配置文件中的日期
			if isStartTime {
				SetCleanStartTime(dateLabel.Text, dateLabel, config)
			} else {
				SetCleanEndTime(dateLabel.Text, dateLabel, config)
			}

			// 关闭对话框
			win.Canvas().Overlays().Top().Hide()
		})
		items = append(items, button)
	}

	grid := container.NewGridWithColumns(columns, items...)
	dialog.ShowCustom(title, "关闭", grid, win)
}

// 更新选择的日期显示
func updateSelectedDate(dateLabel *widget.Label, yearButton, monthButton, dayButton *widget.Button) {
	year, _ := strconv.Atoi(yearButton.Text)
	month, _ := strconv.Atoi(monthButton.Text)
	day, _ := strconv.Atoi(dayButton.Text)
	dateLabel.SetText(formatDate(year, month, day))
}

// 格式化日期为 YYYY.MM.DD
func formatDate(year, month, day int) string {
	return fmt.Sprintf("%d.%02d.%02d", year, month, day) // 保证月、日有前导零
}

// 生成最近 10 年的年份列表
func generateYearList() []string {
	var years []string
	currentYear := time.Now().Year()
	for i := 0; i < 10; i++ {
		years = append(years, strconv.Itoa(currentYear-i))
	}
	return years
}

// 生成月份列表
func generateMonthList() []string {
	var months []string
	for i := 1; i <= 12; i++ {
		months = append(months, fmt.Sprintf("%02d", i))
	}
	return months
}

// 生成某年某月的天数列表
func generateDayList(year int, month int) []string {
	var days []string
	daysInMonth := daysInMonth(year, month)
	for i := 1; i <= daysInMonth; i++ {
		days = append(days, fmt.Sprintf("%02d", i))
	}
	return days
}

// 计算某年某月的天数
func daysInMonth(year int, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
