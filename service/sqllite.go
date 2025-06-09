package service

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bytedance/gopkg/util/logger"
	_ "github.com/mattn/go-sqlite3"
)

// NoteRecord 定义笔记记录结构体
type NoteRecord struct {
	ID        int    `json:"id"`
	NoteID    string `json:"note_id"`
	Content   string `json:"content"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"created_at"`
}

var (
	dbName        = "mowen.db" // 修改为不带路径前缀的文件名
	dbTable       = "mowen"
	sqliteDB      *sql.DB
	sqliteOnce    sync.Once
	sqliteInitErr error
)

// InitSQLite 初始化SQLite数据库连接
func InitSQLite() error {
	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return fmt.Errorf("获取当前工作目录失败: %v", err)
	}

	dbPath := filepath.Join(currentDir, dbName)

	sqliteOnce.Do(func() {
		// 确保数据库文件所在目录存在
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			f, err := os.Create(dbPath)
			if err != nil {
				sqliteInitErr = fmt.Errorf("创建空数据库文件失败: %v", err)
				return
			}
			f.Close()
			logger.Infof("创建空数据库文件成功: %s", dbPath)
		}

		var db *sql.DB
		db, sqliteInitErr = sql.Open("sqlite3", dbPath)
		if sqliteInitErr != nil {
			sqliteInitErr = fmt.Errorf("打开SQLite数据库失败: %v", sqliteInitErr)
			return
		}

		// 测试连接
		sqliteInitErr = db.Ping()
		if sqliteInitErr != nil {
			return
		}

		// 创建表（如果不存在）
		_, sqliteInitErr = db.Exec(fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				note_id TEXT NOT NULL,
				content TEXT NOT NULL,
				summary TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`, dbTable))
		if sqliteInitErr != nil {
			sqliteInitErr = fmt.Errorf("创建表失败: %v", sqliteInitErr)
			return
		}

		sqliteDB = db
		logger.Info("SQLite数据库初始化成功")
	})

	return sqliteInitErr
}

// SaveToSQLite 将数据保存到SQLite数据库
func SaveToSQLite(rows []string) (bool, error) {
	if err := InitSQLite(); err != nil {
		return false, fmt.Errorf("SQLite初始化失败: %v", err)
	}

	if len(rows) == 0 {
		logger.Debug("没有数据需要保存到SQLite")
		return true, nil
	}

	placeholders := make([]string, len(rows))
	args := make([]any, len(rows))
	for i, row := range rows {
		placeholders[i] = "(?)"
		args[i] = row
	}

	insertSQL := fmt.Sprintf("INSERT INTO %s (table_name) VALUES %s",
		dbTable, strings.Join(placeholders, ","))

	_, err := sqliteDB.Exec(insertSQL, args...)
	if err != nil {
		return false, fmt.Errorf("批量插入数据失败: %v", err)
	}
	logger.Infof("成功保存数据到SQLite: %s", insertSQL)
	return true, nil
}

// SearchByDateRange 根据时间段查询
func SearchByDateRange(startDate, endDate string) ([]NoteRecord, error) {
	if err := InitSQLite(); err != nil {
		return nil, fmt.Errorf("SQLite初始化失败: %v", err)
	}

	// 构建查询语句
	query := fmt.Sprintf("SELECT id, note_id, content, summary, created_at FROM %s WHERE created_at BETWEEN ? AND ? ORDER BY created_at DESC", dbTable)

	// 执行查询
	rows, err := sqliteDB.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %v", err)
	}
	defer rows.Close()

	var results []NoteRecord
	for rows.Next() {
		var record NoteRecord
		err = rows.Scan(&record.ID, &record.NoteID, &record.Content, &record.Summary, &record.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("扫描结果失败: %v", err)
		}
		results = append(results, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果失败: %v", err)
	}

	return results, nil
}

// SearchByDate 根据日期查询（支持模糊匹配）
func SearchByDate(date string) ([]NoteRecord, error) {
	if err := InitSQLite(); err != nil {
		return nil, fmt.Errorf("SQLite初始化失败: %v", err)
	}

	// 构建查询语句，支持日期模糊匹配
	query := fmt.Sprintf("SELECT id, note_id, content, summary, created_at FROM %s WHERE DATE(created_at) = DATE(?) ORDER BY created_at DESC", dbTable)

	// 执行查询
	rows, err := sqliteDB.Query(query, date)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %v", err)
	}
	defer rows.Close()

	var results []NoteRecord
	for rows.Next() {
		var record NoteRecord
		err = rows.Scan(&record.ID, &record.NoteID, &record.Content, &record.Summary, &record.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("扫描结果失败: %v", err)
		}
		results = append(results, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果失败: %v", err)
	}

	return results, nil
}

// SearchByCreateDt 根据具体时间查询
func SearchByCreateDt(cdt string) (*NoteRecord, error) {
	if err := InitSQLite(); err != nil {
		return nil, fmt.Errorf("SQLite初始化失败: %v", err)
	}
	// 构建查询语句
	query := fmt.Sprintf("SELECT id, note_id, content, summary, created_at FROM %s WHERE created_at = ?", dbTable)
	// 执行查询
	var record NoteRecord
	err := sqliteDB.QueryRow(query, cdt).Scan(&record.ID, &record.NoteID, &record.Content, &record.Summary, &record.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("未找到匹配的记录")
		}
		return nil, fmt.Errorf("查询失败: %v", err)
	}
	return &record, nil
}

// CloseSQLite 关闭SQLite数据库连接
func CloseSQLite() {
	if sqliteDB != nil {
		logger.Info("关闭SQLite数据库连接")
		sqliteDB.Close()
		sqliteDB = nil
	}
}

// SaveNoteToSQLite 将笔记数据保存到SQLite数据库
func SaveNoteToSQLite(noteID, content, summary string) (bool, error) {
	if err := InitSQLite(); err != nil {
		return false, fmt.Errorf("SQLite初始化失败: %v", err)
	}

	if noteID == "" || content == "" {
		logger.Debug("笔记ID或内容为空，跳过保存")
		return false, fmt.Errorf("笔记ID和内容不能为空")
	}

	// 构建插入SQL语句
	insertSQL := fmt.Sprintf("INSERT INTO %s (note_id, content, summary) VALUES (?, ?, ?)", dbTable)

	// 执行插入
	_, err := sqliteDB.Exec(insertSQL, noteID, content, summary)
	if err != nil {
		return false, fmt.Errorf("保存笔记数据失败: %v", err)
	}

	logger.Infof("成功保存笔记数据到SQLite，noteID: %s, contentLength: %d", noteID, len(content))
	return true, nil
}
