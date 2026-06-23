package store

import (
	"fmt"
	"time"

	"go-agent-chat-server/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitMySQL(dsn string) error {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect mysql failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db failed: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := db.AutoMigrate(&model.User{}, &model.Session{}, &model.Message{}, &model.KnowledgeDoc{}, &model.ChatJob{}); err != nil {
		return fmt.Errorf("auto migrate failed: %w", err)
	}

	DB = db
	return nil
}
