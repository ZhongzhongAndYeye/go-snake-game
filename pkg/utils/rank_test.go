package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-snake-game/pkg/config"
	"go-snake-game/pkg/db"
)

// setupRankTest 初始化测试环境：加载配置、连接 Redis、清理测试数据。
func setupRankTest(t *testing.T) {
	t.Helper()

	// 加载配置
	err := config.InitConfig("../../configs/dev.yaml")
	if err != nil {
		t.Fatalf("加载配置文件失败: %v", err)
	}

	// 初始化 Redis
	redisCfg := config.GlobalCfg.Redis
	db.InitRedis(&db.RedisConfig{
		Addr:         redisCfg.Addr,
		DB:           redisCfg.DB,
		Password:     redisCfg.Password,
		PoolSize:     redisCfg.PoolSize,
		MinIdleConns: redisCfg.MinIdleConns,
		MaxRetries:   redisCfg.MaxRetries,
		DialTimeout:  redisCfg.DialTimeout,
		ReadTimeout:  redisCfg.ReadTimeout,
		WriteTimeout: redisCfg.WriteTimeout,
		PoolTimeout:  redisCfg.PoolTimeout,
	})

	// 测试前清理排行榜数据，确保测试环境干净
	cleanRank(t)
}

// cleanRank 清理排行榜测试数据。
func cleanRank(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.GlobalRedis.Del(ctx, globalRankKey).Err(); err != nil {
		t.Fatalf("清理排行榜数据失败: %v", err)
	}
}

// TestAddPlayerScore 测试写入玩家得分。
func TestAddPlayerScore(t *testing.T) {
	setupRankTest(t)

	// 写入三个玩家不同分数
	err := AddPlayerScore(1001, 50)
	if err != nil {
		t.Fatalf("写入玩家 1001 得分失败: %v", err)
	}

	err = AddPlayerScore(1002, 80)
	if err != nil {
		t.Fatalf("写入玩家 1002 得分失败: %v", err)
	}

	err = AddPlayerScore(1003, 30)
	if err != nil {
		t.Fatalf("写入玩家 1003 得分失败: %v", err)
	}

	// 验证写入成功：查询排行榜条目数
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	count, err := db.GlobalRedis.ZCard(ctx, globalRankKey).Result()
	if err != nil {
		t.Fatalf("查询排行榜数量失败: %v", err)
	}
	if count != 3 {
		t.Fatalf("排行榜数量期望 3，实际 %d", count)
	}

	t.Log("✅ AddPlayerScore: 写入三个玩家得分成功")
}

// TestAddPlayerScoreUpdate 测试更新玩家得分（覆盖旧分数）。
func TestAddPlayerScoreUpdate(t *testing.T) {
	setupRankTest(t)

	// 先写入初始分数
	err := AddPlayerScore(1001, 50)
	if err != nil {
		t.Fatalf("写入初始分数失败: %v", err)
	}

	// 更新为更高分数
	err = AddPlayerScore(1001, 90)
	if err != nil {
		t.Fatalf("更新分数失败: %v", err)
	}

	// 验证分数已更新，且排行榜仍只有 1 个成员
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	score, err := db.GlobalRedis.ZScore(ctx, globalRankKey, "1001").Result()
	if err != nil {
		t.Fatalf("查询分数失败: %v", err)
	}
	if int(score) != 90 {
		t.Fatalf("分数期望 90，实际 %d", int(score))
	}

	count, err := db.GlobalRedis.ZCard(ctx, globalRankKey).Result()
	if err != nil {
		t.Fatalf("查询排行榜数量失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("排行榜数量期望 1，实际 %d", count)
	}

	t.Log("✅ AddPlayerScoreUpdate: 更新玩家得分成功（ZSet 去重覆盖）")
}

// TestGetTopN 测试查询排行榜前 N 名。
func TestGetTopN(t *testing.T) {
	setupRankTest(t)

	// 准备测试数据：按分数从高到低排序为 1003(90)、1001(50)、1002(30)
	_ = AddPlayerScore(1001, 50)
	_ = AddPlayerScore(1002, 30)
	_ = AddPlayerScore(1003, 90)

	// 查询前 2 名
	items, err := GetTopN(2)
	if err != nil {
		t.Fatalf("查询 Top2 失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("Top2 结果数量期望 2，实际 %d", len(items))
	}

	// 第 1 名：1003，分数 90
	if items[0].PlayerID != 1003 || items[0].Score != 90 || items[0].Rank != 1 {
		t.Fatalf("第 1 名期望 player_id=1003, score=90, rank=1，实际 player_id=%d, score=%d, rank=%d",
			items[0].PlayerID, items[0].Score, items[0].Rank)
	}

	// 第 2 名：1001，分数 50
	if items[1].PlayerID != 1001 || items[1].Score != 50 || items[1].Rank != 2 {
		t.Fatalf("第 2 名期望 player_id=1001, score=50, rank=2，实际 player_id=%d, score=%d, rank=%d",
			items[1].PlayerID, items[1].Score, items[1].Rank)
	}

	t.Log("✅ GetTopN: 查询前 2 名，顺序正确，排名从 1 开始")
}

// TestGetTopNFull 测试查询全部排名（N 大于实际人数）。
func TestGetTopNFull(t *testing.T) {
	setupRankTest(t)

	_ = AddPlayerScore(1001, 50)
	_ = AddPlayerScore(1002, 80)

	// 查询前 10 名，但只有 2 个玩家
	items, err := GetTopN(10)
	if err != nil {
		t.Fatalf("查询 Top10 失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("Top10 但只有 2 个玩家，结果数量期望 2，实际 %d", len(items))
	}

	t.Log("✅ GetTopNFull: N 大于实际人数时返回全部已有数据")
}

// TestGetTopNEmpty 测试空排行榜返回空切片。
func TestGetTopNEmpty(t *testing.T) {
	setupRankTest(t)

	items, err := GetTopN(10)
	if err != nil {
		t.Fatalf("查询空排行榜失败: %v", err)
	}
	if items == nil {
		t.Fatal("空排行榜应返回空切片而非 nil")
	}
	if len(items) != 0 {
		t.Fatalf("空排行榜结果数量期望 0，实际 %d", len(items))
	}

	t.Log("✅ GetTopNEmpty: 空排行榜返回空切片")
}

// TestGetPlayerRank 测试查询玩家排名和得分。
func TestGetPlayerRank(t *testing.T) {
	setupRankTest(t)

	// 准备数据：1003(90)、1001(50)、1002(30)
	_ = AddPlayerScore(1001, 50)
	_ = AddPlayerScore(1002, 30)
	_ = AddPlayerScore(1003, 90)

	// 查询第 1 名
	rank, score, err := GetPlayerRank(1003)
	if err != nil {
		t.Fatalf("查询玩家 1003 排名失败: %v", err)
	}
	if rank != 1 || score != 90 {
		t.Fatalf("玩家 1003 期望 rank=1, score=90，实际 rank=%d, score=%d", rank, score)
	}

	// 查询第 2 名
	rank, score, err = GetPlayerRank(1001)
	if err != nil {
		t.Fatalf("查询玩家 1001 排名失败: %v", err)
	}
	if rank != 2 || score != 50 {
		t.Fatalf("玩家 1001 期望 rank=2, score=50，实际 rank=%d, score=%d", rank, score)
	}

	// 查询第 3 名
	rank, score, err = GetPlayerRank(1002)
	if err != nil {
		t.Fatalf("查询玩家 1002 排名失败: %v", err)
	}
	if rank != 3 || score != 30 {
		t.Fatalf("玩家 1002 期望 rank=3, score=30，实际 rank=%d, score=%d", rank, score)
	}

	t.Log("✅ GetPlayerRank: 三个玩家排名和得分查询正确")
}

// TestGetPlayerRankNotFound 测试查询不存在的玩家返回 ErrPlayerNotFound。
func TestGetPlayerRankNotFound(t *testing.T) {
	setupRankTest(t)

	// 排行榜为空，查询不存在的玩家
	rank, score, err := GetPlayerRank(9999)
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("查询不存在的玩家期望 ErrPlayerNotFound，实际 err=%v, rank=%d, score=%d", err, rank, score)
	}

	// 排行榜有数据，查询不存在的玩家
	_ = AddPlayerScore(1001, 50)
	rank, score, err = GetPlayerRank(9999)
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("查询不存在的玩家期望 ErrPlayerNotFound，实际 err=%v, rank=%d, score=%d", err, rank, score)
	}

	t.Log("✅ GetPlayerRankNotFound: 不存在的玩家正确返回 ErrPlayerNotFound")
}

// TestCleanup 测试结束后清理排行榜数据。
func TestCleanup(t *testing.T) {
	// 这个测试用例仅用于清理测试产生的排行榜数据
	// 在测试套件最后手动清理
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = db.GlobalRedis.Del(ctx, globalRankKey).Err()
	t.Log("✅ 排行榜测试数据已清理")
}
