package utils

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db   *sql.DB
	once sync.Once
)

// DBConfig 数据库配置
type DBConfig struct {
	DBPath string // 数据库文件路径
}

// InitDB 初始化数据库连接
func InitDB(config *DBConfig) error {
	var err error

	once.Do(func() {
		// 确保数据库目录存在
		dbDir := filepath.Dir(config.DBPath)
		if err = os.MkdirAll(dbDir, os.ModePerm); err != nil {
			return
		}

		// 连接数据库
		db, err = sql.Open("sqlite3", config.DBPath)
		if err != nil {
			return
		}

		// 测试连接
		if err = db.Ping(); err != nil {
			return
		}

		// 创建必要的表
		err = createTables()
	})

	return err
}

// GetDB 获取数据库连接
func GetDB() *sql.DB {
	return db
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// createTables 创建必要的表
func createTables() error {
	// 创建文件复制记录表
	_, err := db.Exec(`
    CREATE TABLE IF NOT EXISTS copy_records (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        file_name TEXT NOT NULL UNIQUE,
        copy_dir TEXT NOT NULL,
        copy_time TIMESTAMP NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("创建文件复制记录表失败: %v", err)
	}

	return nil
}

// CheckFileExists 检查文件是否已经复制过
func CheckFileExists(fileName string, isAutotask bool) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM copy_records WHERE file_name =?", fileName).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// DeleteFileCopyRecord 删除文件复制记录
func DeleteFileCopyRecord(fileName string, isAutotask bool) error {
	_, err := db.Exec("DELETE FROM copy_records WHERE file_name =?", fileName)
	return err
}

// RecordFileCopy 记录文件复制操作
func RecordFileCopy(fileName string, copyDir string, isAutoTask bool) error {
	_, err := db.Exec(
		`INSERT OR REPLACE INTO copy_records (file_name, copy_dir, copy_time) 
        VALUES (?, ?, ?)`,
		fileName, copyDir, time.Now())
	return err
}

// ExecDB 执行SQL语句并返回结果
func ExecDB(query string, args ...interface{}) (sql.Result, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	return db.Exec(query, args...)
}

// QueryRowDB 执行查询并返回单行结果
func QueryRowDB(query string, args ...interface{}) *sql.Row {
	if db == nil {
		return nil
	}
	return db.QueryRow(query, args...)
}
