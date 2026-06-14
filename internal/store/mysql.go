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

	if err := db.AutoMigrate(&model.User{}, &model.Session{}, &model.Message{}, &model.KnowledgeDoc{}); err != nil {
		return fmt.Errorf("auto migrate failed: %w", err)
	} //根据Go里的model结构体，自动在MySQL里创建或更新对应的数据表结构

	DB = db
	return nil
}
