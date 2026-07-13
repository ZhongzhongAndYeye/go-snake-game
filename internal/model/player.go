package model

import (
	"time"
)

// Player 玩家数据模型
type Player struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	Username     string    `gorm:"column:username;size:32;uniqueIndex;not null"`
	PasswordHash string    `gorm:"column:password_hash;size:128;not null"`
	Nickname     string    `gorm:"column:nickname;size:64;not null"`
	MaxScore     int       `gorm:"column:max_score;default:0;not null"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定模型对应的数据库表名。
func (Player) TableName() string {
	return "players"
}
