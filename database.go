package main

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"database/sql"

	// for sqlite3
	_ "github.com/mattn/go-sqlite3"
)

// constants for local database
const (
	DBFilename = "db.sqlite"
)

// Database struct
type Database struct {
	db *sql.DB
	sync.RWMutex
}

// Log struct
type Log struct {
	Type    string
	Message string
	Time    time.Time
}

// Chat struct
type Chat struct {
	ChatID int
	UserID string
	Time   time.Time
}

var _db *Database = nil

// OpenDB opens database
func OpenDB() *Database {
	if _db == nil {
		if execFilepath, err := os.Executable(); err != nil {
			panic(err)
		} else {
			if db, err := sql.Open("sqlite3", filepath.Join(filepath.Dir(execFilepath), DBFilename)); err != nil {
				panic("Failed to open database: " + err.Error())
			} else {
				_db = &Database{
					db: db,
				}

				// logs table
				if _, err := db.Exec(`create table if not exists logs(
					id integer primary key autoincrement,
					type text default null,
					message text not null,
					time datetime default current_timestamp
				)`); err != nil {
					panic("Failed to create logs table: " + err.Error())
				}

				// chats table
				if _, err := db.Exec(`create table if not exists chats(
					id integer primary key autoincrement,
					chat_id integer not null,
					user_id text not null,
					create_time datetime default current_timestamp,
					unique(chat_id)
				)`); err != nil {
					panic("Failed to create chats table: " + err.Error())
				}
			}
		}
	}

	return _db
}

// CloseDB closes database
func CloseDB() {
	if _db != nil {
		_db.db.Close()
		_db = nil
	}
}

func (d *Database) saveLog(typ, msg string) {
	d.Lock()

	if stmt, err := d.db.Prepare(`insert into logs(type, message) values(?, ?)`); err != nil {
		log.Printf("*** failed to prepare a statement: %s", err.Error())
	} else {
		defer stmt.Close()
		if _, err = stmt.Exec(typ, msg); err != nil {
			log.Printf("*** failed to save log into local database: %s", err.Error())
		}
	}

	d.Unlock()
}

// Log logs a message
func (d *Database) Log(msg string) {
	d.saveLog("log", msg)
}

// LogError logs an error message
func (d *Database) LogError(msg string) {
	d.saveLog("err", msg)
}

// GetLogs fetches logs
func (d *Database) GetLogs(latestN int) []Log {
	logs := []Log{}

	d.RLock()

	if stmt, err := d.db.Prepare(`select type, message, datetime(time, 'localtime') as time from logs order by id desc limit ?`); err != nil {
		log.Printf("*** failed to prepare a statement: %s", err.Error())
	} else {
		defer stmt.Close()

		if rows, err := stmt.Query(latestN); err != nil {
			log.Printf("*** failed to select logs from local database: %s", err.Error())
		} else {
			defer rows.Close()

			var typ, msg, datetime string
			var tm time.Time
			for rows.Next() {
				if err := rows.Scan(&typ, &msg, &datetime); err == nil {
					tm, _ = time.Parse("2006-01-02 15:04:05", datetime)

					logs = append(logs, Log{
						Type:    typ,
						Message: msg,
						Time:    tm,
					})
				} else {
					log.Printf("*** failed to scan row: %s", err.Error())
				}
			}
		}
	}

	d.RUnlock()

	return logs
}

// SaveChat saves chat
func (d *Database) SaveChat(chatID int64, userID string) {
	d.Lock()

	if stmt, err := d.db.Prepare(`insert or ignore into chats(chat_id, user_id) values(?, ?)`); err != nil {
		log.Printf("*** failed to prepare a statement: %s", err.Error())
	} else {
		defer stmt.Close()
		if _, err = stmt.Exec(chatID, userID); err != nil {
			log.Printf("*** failed to save chat into local database: %s", err.Error())
		}
	}

	d.Unlock()
}

// DeleteChat deletes chat
func (d *Database) DeleteChat(chatID int) {
	d.Lock()

	if stmt, err := d.db.Prepare(`delete from chats where chat_id = ?`); err != nil {
		log.Printf("*** failed to prepare a statement: %s", err.Error())
	} else {
		defer stmt.Close()
		if _, err = stmt.Exec(chatID); err != nil {
			log.Printf("*** failed to delete chat from local database: %s", err.Error())
		}
	}

	d.Unlock()
}

// GetChats retrieves chats
func (d *Database) GetChats() []Chat {
	chats := []Chat{}

	d.RLock()

	if stmt, err := d.db.Prepare(`select chat_id, user_id, datetime(create_time, 'localtime') as time from chats`); err != nil {
		log.Printf("*** failed to prepare a statement: %s", err.Error())
	} else {
		defer stmt.Close()

		if rows, err := stmt.Query(); err != nil {
			log.Printf("*** failed to select chats from local database: %s", err.Error())
		} else {
			defer rows.Close()

			var chatID int
			var userID, datetime string
			var tm time.Time
			for rows.Next() {
				if err := rows.Scan(&chatID, &userID, &datetime); err == nil {
					tm, _ = time.Parse("2006-01-02 15:04:05", datetime)

					chats = append(chats, Chat{
						ChatID: chatID,
						UserID: userID,
						Time:   tm,
					})
				} else {
					log.Printf("*** failed to scan row: %s", err.Error())
				}
			}
		}
	}

	d.RUnlock()

	return chats
}
