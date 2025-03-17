package database

import (
	"fmt"
	"time"
)

// CopyRecord 复制记录结构体
type CopyRecord struct {
	ID        int64
	FileName  string
	CopyDir   string
	DateRange string // 仅计划任务有
	CopyTime  time.Time
	Status    string
}

// GetAutoCopyRecords 获取自动任务复制记录
func GetAutoCopyRecords(date string, limit, offset int) ([]CopyRecord, error) {
	query := `
        SELECT id, file_name, copy_dir, copy_time, status
        FROM auto_copy_records
        WHERE 1=1
    `
	var args []interface{}

	if date != "" {
		query += " AND copy_dir = ?"
		args = append(args, date)
	}

	query += " ORDER BY copy_time DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []CopyRecord
	for rows.Next() {
		var record CopyRecord
		if err := rows.Scan(
			&record.ID, &record.FileName, &record.CopyDir,
			&record.CopyTime, &record.Status); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// GetScheduledCopyRecords 获取计划任务复制记录
func GetScheduledCopyRecords(startDate, endDate string, limit, offset int) ([]CopyRecord, error) {
	query := `
        SELECT id, file_name, copy_dir, date_range, copy_time, status
        FROM scheduled_copy_records
        WHERE 1=1
    `
	var args []interface{}

	if startDate != "" && endDate != "" {
		// 提取日期范围中的日期
		query += " AND (date_range LIKE ? OR copy_dir BETWEEN ? AND ?)"
		dateRangePattern := "%" + startDate + "%" + endDate + "%"
		args = append(args, dateRangePattern, startDate, endDate)
	} else if startDate != "" {
		query += " AND copy_dir >= ?"
		args = append(args, startDate)
	} else if endDate != "" {
		query += " AND copy_dir <= ?"
		args = append(args, endDate)
	}

	query += " ORDER BY copy_time DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []CopyRecord
	for rows.Next() {
		var record CopyRecord
		if err := rows.Scan(
			&record.ID, &record.FileName, &record.CopyDir, &record.DateRange,
			&record.CopyTime, &record.Status); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// SearchCopyRecords 按文件名搜索记录
func SearchCopyRecords(keyword string, isAutoTask bool, limit, offset int) ([]CopyRecord, error) {
	var tableName string
	if isAutoTask {
		tableName = "auto_copy_records"
	} else {
		tableName = "scheduled_copy_records"
	}

	query := fmt.Sprintf(`
        SELECT id, file_name, copy_dir, %s copy_time, status
        FROM %s
        WHERE file_name LIKE ?
        ORDER BY copy_time DESC
        LIMIT ? OFFSET ?
    `,
		// 如果是计划任务,添加date_range字段
		func() string {
			if !isAutoTask {
				return "date_range,"
			}
			return ""
		}(),
		tableName)

	pattern := "%" + keyword + "%"

	rows, err := db.Query(query, pattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []CopyRecord
	for rows.Next() {
		var record CopyRecord
		var scanArgs []interface{}

		scanArgs = append(scanArgs, &record.ID, &record.FileName, &record.CopyDir)
		if !isAutoTask {
			scanArgs = append(scanArgs, &record.DateRange)
		}
		scanArgs = append(scanArgs, &record.CopyTime, &record.Status)

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}
