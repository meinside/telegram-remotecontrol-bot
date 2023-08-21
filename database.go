package main

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/meinside/telegram-remotecontrol-bot/cfg"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// constants for local database
const (
	dbFilename = "db.sqlite"
)

// Database struct
type Database struct {
	db *gorm.DB
}

// Log struct
type Log struct {
	gorm.Model

	Type    string
	Message string
}

// Chat struct
type Chat struct {
	gorm.Model

	ChatID int64 `gorm:"uniqueIndex"`
	UserID string
}

// OpenDB opens database and returns it
func OpenDB() (database *Database, err error) {
	var configDir string
	configDir, err = cfg.GetConfigDir()

	if err == nil {
		dbFilepath := filepath.Join(configDir, dbFilename)

		var db *gorm.DB
		if db, err = gorm.Open(sqlite.Open(dbFilepath), &gorm.Config{}); err != nil {
			err = fmt.Errorf("gorm failed to open database: %s", err)
		} else {
			// migrate tables
			if err = db.AutoMigrate(&Log{}, &Chat{}); err == nil {
				return &Database{db: db}, nil
			} else {
				err = fmt.Errorf("gorm failed to migrate database: %s", err)
			}
		}
	}

	return nil, err
}

// save log
func (d *Database) saveLog(typ, msg string) {
	if tx := d.db.Create(&Log{Type: typ, Message: msg}); tx.Error != nil {
		log.Printf("* failed to save log into local database: %s", tx.Error)
	}
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
func (d *Database) GetLogs(latestN int) (result []Log) {
	if tx := d.db.Order("id desc").Limit(latestN).Find(&result); tx.Error != nil {
		log.Printf("* failed to get logs from local database: %s", tx.Error)

		return []Log{}
	}

	return result
}

// SaveChat saves chat
func (d *Database) SaveChat(chatID int64, userID string) {
	if tx := d.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Chat{ChatID: chatID, UserID: userID}); tx.Error != nil {
		log.Printf("* failed to save chat into local database: %s", tx.Error)
	}
}

// DeleteChat deletes chat
func (d *Database) DeleteChat(chatID int) {
	if tx := d.db.Where("chat_id = ?", chatID).Delete(&Chat{}); tx.Error != nil {
		log.Printf("* failed to delete chat from local database: %s", tx.Error)
	}
}

// GetChats retrieves chats
func (d *Database) GetChats() (result []Chat) {
	if tx := d.db.Find(&result); tx.Error != nil {
		log.Printf("* failed to get chats from local database: %s", tx.Error)

		return []Chat{}
	}

	return result
}
