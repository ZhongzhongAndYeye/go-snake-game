package dao

import (
	"testing"

	"go-snake-game/internal/model"
	"go-snake-game/pkg/config"
	"go-snake-game/pkg/db"
	"go-snake-game/pkg/logger"
)

// setupDB 初始化数据库连接和表结构，供测试使用。
func setupDB(t *testing.T) {
	t.Helper()

	// 初始化日志
	_ = logger.InitLogger(config.LogConfig{
		Level:   "error",
		Console: false,
	})

	// 加载配置文件
	err := config.InitConfig("../../configs/dev.yaml")
	if err != nil {
		t.Fatalf("加载配置文件失败: %v", err)
	}

	// 初始化 MySQL
	cfg := config.GlobalCfg.Mysql
	db.InitMySQL(&db.MySQLConfig{
		DSN:            cfg.DSN,
		MaxOpen:        cfg.MaxOpen,
		MaxIdle:        cfg.MaxIdle,
		MaxLifeMinutes: cfg.MaxLifeMinutes,
	})

	// 自动迁移表结构
	err = db.GlobalDB.AutoMigrate(&model.Player{})
	if err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
}

// cleanupPlayer 删除测试创建的玩家数据。
func cleanupPlayer(t *testing.T, username string) {
	t.Helper()
	db.GlobalDB.Where("username = ?", username).Delete(&model.Player{})
}

// TestCreatePlayer 测试创建玩家。
func TestCreatePlayer(t *testing.T) {
	setupDB(t)
	defer cleanupPlayer(t, "test_create")

	player := &model.Player{
		Username:     "test_create",
		PasswordHash: "$2a$10$hash_value_for_test",
		Nickname:     "测试创建",
	}

	err := CreatePlayer(player)
	if err != nil {
		t.Fatalf("CreatePlayer 失败: %v", err)
	}

	if player.ID == 0 {
		t.Fatal("CreatePlayer 后 ID 为空，自增主键未赋值")
	}

	t.Logf("创建玩家成功，ID: %d", player.ID)
}

// TestGetPlayerByUsername 测试根据用户名查询玩家。
func TestGetPlayerByUsername(t *testing.T) {
	setupDB(t)

	username := "test_get_by_username"
	defer cleanupPlayer(t, username)

	// 先创建玩家
	player := &model.Player{
		Username:     username,
		PasswordHash: "$2a$10$hash_value_for_test",
		Nickname:     "测试查询用户名",
	}
	err := CreatePlayer(player)
	if err != nil {
		t.Fatalf("CreatePlayer 失败: %v", err)
	}

	// 查询存在的玩家
	found, err := GetPlayerByUsername(username)
	if err != nil {
		t.Fatalf("GetPlayerByUsername 失败: %v", err)
	}
	if found.Username != username {
		t.Errorf("用户名不匹配，期望 %s，实际 %s", username, found.Username)
	}
	if found.Nickname != "测试查询用户名" {
		t.Errorf("昵称不匹配，期望 %s，实际 %s", "测试查询用户名", found.Nickname)
	}

	// 查询不存在的玩家
	_, err = GetPlayerByUsername("not_exist_user")
	if err != ErrPlayerNotFound {
		t.Errorf("不存在的用户应返回 ErrPlayerNotFound，实际: %v", err)
	}
}

// TestGetPlayerByID 测试根据 ID 查询玩家。
func TestGetPlayerByID(t *testing.T) {
	setupDB(t)

	username := "test_get_by_id"
	defer cleanupPlayer(t, username)

	// 先创建玩家
	player := &model.Player{
		Username:     username,
		PasswordHash: "$2a$10$hash_value_for_test",
		Nickname:     "测试查询ID",
	}
	err := CreatePlayer(player)
	if err != nil {
		t.Fatalf("CreatePlayer 失败: %v", err)
	}

	// 查询存在的玩家
	found, err := GetPlayerByID(player.ID)
	if err != nil {
		t.Fatalf("GetPlayerByID 失败: %v", err)
	}
	if found.ID != player.ID {
		t.Errorf("ID 不匹配，期望 %d，实际 %d", player.ID, found.ID)
	}

	// 查询不存在的 ID
	_, err = GetPlayerByID(999999)
	if err != ErrPlayerNotFound {
		t.Errorf("不存在的 ID 应返回 ErrPlayerNotFound，实际: %v", err)
	}
}

// TestCreateDuplicateUsername 测试创建重复用户名应失败。
func TestCreateDuplicateUsername(t *testing.T) {
	setupDB(t)

	username := "test_dup"
	defer cleanupPlayer(t, username)

	// 创建第一个玩家
	player1 := &model.Player{
		Username:     username,
		PasswordHash: "$2a$10$hash1",
		Nickname:     "玩家1",
	}
	err := CreatePlayer(player1)
	if err != nil {
		t.Fatalf("第一次创建玩家失败: %v", err)
	}

	// 创建相同用户名的第二个玩家，应失败
	player2 := &model.Player{
		Username:     username,
		PasswordHash: "$2a$10$hash2",
		Nickname:     "玩家2",
	}
	err = CreatePlayer(player2)
	if err == nil {
		t.Fatal("重复用户名创建应失败，但成功了")
	}
	t.Logf("重复用户名创建失败，正确返回错误: %v", err)
}

// TestUpdatePlayerScore 测试更新玩家最高分。
func TestUpdatePlayerScore(t *testing.T) {
	setupDB(t)

	username := "test_score"
	defer cleanupPlayer(t, username)

	// 创建玩家
	player := &model.Player{
		Username:     username,
		PasswordHash: "$2a$10$hash",
		Nickname:     "测试分数",
	}
	err := CreatePlayer(player)
	if err != nil {
		t.Fatalf("CreatePlayer 失败: %v", err)
	}

	// 更新分数（新分数更高，应成功）
	err = UpdatePlayerScore(player.ID, 100)
	if err != nil {
		t.Fatalf("UpdatePlayerScore(100) 失败: %v", err)
	}

	// 验证分数已更新
	found, _ := GetPlayerByID(player.ID)
	if found.MaxScore != 100 {
		t.Errorf("分数不匹配，期望 100，实际 %d", found.MaxScore)
	}

	// 更新更低分数（不应更新）
	err = UpdatePlayerScore(player.ID, 50)
	if err != nil && err != ErrPlayerNotFound {
		t.Fatalf("UpdatePlayerScore(50) 返回错误: %v", err)
	}

	// 验证分数不变
	found, _ = GetPlayerByID(player.ID)
	if found.MaxScore != 100 {
		t.Errorf("低分不应更新，期望 100，实际 %d", found.MaxScore)
	}

	// 更新不存在的玩家
	err = UpdatePlayerScore(999999, 200)
	if err != ErrPlayerNotFound {
		t.Errorf("不存在的玩家应返回 ErrPlayerNotFound，实际: %v", err)
	}
}