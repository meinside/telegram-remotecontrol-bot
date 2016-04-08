package helper

import (
	"log"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

const (
	// constants for local database
	DbFilename = "../db.sqlite"
)

type Database struct {
	db *sql.DB
	sync.RWMutex
}

type Log struct {
	Type    string
	Message string
	Time    time.Time
}

type Chat struct {
	ChatId int
	UserId string
	Time   time.Time
}

var _db *Database = nil

func OpenDb() *Database {
	if _db == nil {
		_, filename, _, _ := runtime.Caller(0) // = __FILE__

		if db, err := sql.Open("sqlite3", filepath.Join(path.Dir(filename), DbFilename)); err != nil {
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

	return _db
}

func CloseDb() {
	if _db != nil {
		_db.db.Close()
		_db = nil
	}
}

func (d *Database) saveLog(typ, msg string) {
	d.Lock()

	if stmt, err := d.db.Prepare(`insert into logs(type, message) values(?, ?)`); err != nil {
		log.Printf("*** Failed to prepare a statement: %s\n", err.Error())
	} else {
		defer stmt.Close()
		if _, err = stmt.Exec(typ, msg); err != nil {
			log.Printf("*** Failed to save log into local database: %s\n", err.Error())
		}
	}

	d.Unlock()
}

func (d *Database) Log(msg string) {
	d.saveLog("log", msg)
}

func (d *Database) LogError(msg string) {
	d.saveLog("err", msg)
}

func (d *Database) GetLogs(latestN int) []Log {
	logs := []Log{}

	d.RLock()

	if stmt, err := d.db.Prepare(`select type, message, datetime(time, 'localtime') as time from logs order by id desc limit ?`); err != nil {
		log.Printf("*** Failed to prepare a statement: %s\n", err.Error())
	} else {
		defer stmt.Close()

		if rows, err := stmt.Query(latestN); err != nil {
			log.Printf("*** Failed to select logs from local database: %s\n", err.Error())
		} else {
			defer rows.Close()

			var typ, msg, datetime string
			var tm time.Time
			for rows.Next() {
				rows.Scan(&typ, &msg, &datetime)
				tm, _ = time.Parse("2006-01-02 15:04:05", datetime)

				logs = append(logs, Log{
					Type:    typ,
					Message: msg,
					Time:    tm,
				})
			}
		}
	}

	d.RUnlock()

	return logs
}

func (d *Database) SaveChat(chatId int, userId string) {
	d.Lock()

	if stmt, err := d.db.Prepare(`insert or ignore into chats(chat_id, user_id) values(?, ?)`); err != nil {
		log.Printf("*** Failed to prepare a statement: %s\n", err.Error())
	} else {
		defer stmt.Close()
		if _, err = stmt.Exec(chatId, userId); err != nil {
			log.Printf("*** Failed to save chat into local database: %s\n", err.Error())
		}
	}

	d.Unlock()
}

func (d *Database) DeleteChat(chatId int) {
	d.Lock()

	if stmt, err := d.db.Prepare(`delete from chats where chat_id = ?`); err != nil {
		log.Printf("*** Failed to prepare a statement: %s\n", err.Error())
	} else {
		defer stmt.Close()
		if _, err = stmt.Exec(chatId); err != nil {
			log.Printf("*** Failed to delete chat from local database: %s\n", err.Error())
		}
	}

	d.Unlock()
}

func (d *Database) GetChats() []Chat {
	chats := []Chat{}

	d.RLock()

	if stmt, err := d.db.Prepare(`select chat_id, user_id, datetime(create_time, 'localtime') as time from chats`); err != nil {
		log.Printf("*** Failed to prepare a statement: %s\n", err.Error())
	} else {
		defer stmt.Close()

		if rows, err := stmt.Query(); err != nil {
			log.Printf("*** Failed to select chats from local database: %s\n", err.Error())
		} else {
			defer rows.Close()

			var chatId int
			var userId, datetime string
			var tm time.Time
			for rows.Next() {
				rows.Scan(&chatId, &userId, &datetime)
				tm, _ = time.Parse("2006-01-02 15:04:05", datetime)

				chats = append(chats, Chat{
					ChatId: chatId,
					UserId: userId,
					Time:   tm,
				})
			}
		}
	}

	d.RUnlock()

	return chats
}
