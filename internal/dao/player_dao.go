// Package dao 提供玩家数据访问层，封装玩家相关的数据库增删改查操作。
package dao

import (
	"errors"

	"go-snake-game/internal/model"
	"go-snake-game/pkg/db"

	"gorm.io/gorm"
)

var (
	ErrPlayerNotFound = errors.New("玩家不存在")
)

// GetPlayerByUsername 根据用户名查询玩家信息。
// 参数 username: 用户名
// 返回: 玩家对象指针和可能的错误
//   - 查询成功返回玩家对象，error 为 nil
//   - 用户不存在返回 ErrPlayerNotFound
//   - 数据库异常返回原始错误
func GetPlayerByUsername(username string) (*model.Player, error) {
	var player model.Player

	err := db.GlobalDB.Where("username = ?", username).First(&player).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlayerNotFound
		}
		return nil, err
	}

	return &player, nil
}

// GetPlayerByID 根据玩家 ID 查询玩家信息。
// 参数 playerID: 玩家 ID
// 返回: 玩家对象指针和可能的错误
//   - 查询成功返回玩家对象，error 为 nil
//   - 用户不存在返回 ErrPlayerNotFound
//   - 数据库异常返回原始错误
func GetPlayerByID(playerID uint64) (*model.Player, error) {
	var player model.Player

	err := db.GlobalDB.First(&player, playerID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPlayerNotFound
		}
		return nil, err
	}

	return &player, nil
}

// CreatePlayer 创建新玩家。
// 参数 player: 玩家对象指针，包含用户名、密码哈希、昵称等信息
// 返回: 可能的错误（如用户名重复、数据库异常等）
func CreatePlayer(player *model.Player) error {
	return db.GlobalDB.Create(player).Error
}

// UpdatePlayerScore 更新玩家最高分。
// 只有当新分数大于当前最高分才会更新。
// 如果玩家不存在返回 ErrPlayerNotFound，如果分数未超过当前最高分则静默忽略。
// 参数 playerID: 玩家 ID
// 参数 score: 新的分数
// 返回: 可能的错误（如用户不存在、数据库异常等）
func UpdatePlayerScore(playerID uint64, score int) error {
	// 先检查玩家是否存在
	var count int64
	if err := db.GlobalDB.Model(&model.Player{}).Where("id = ?", playerID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrPlayerNotFound
	}

	// 只有当新分数大于当前最高分时才更新
	result := db.GlobalDB.Model(&model.Player{}).
		Where("id = ? AND max_score < ?", playerID, score).
		Update("max_score", score)

	if result.Error != nil {
		return result.Error
	}

	return nil
}
