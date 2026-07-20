package utils

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go-snake-game/pkg/db"
	"go-snake-game/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// 全局排行榜 Redis key，固定前缀，所有玩家共享同一个排行榜。
const globalRankKey = "game:rank:global"

// 预定义的错误变量，供调用方判断错误类型
var (
	ErrPlayerNotFound = errors.New("玩家不存在或未上榜") // 玩家在排行榜中不存在
)

// RankItem 排行榜条目，表示一个玩家的排名与得分。
type RankItem struct {
	PlayerID uint64 // 玩家 ID
	Score    int    // 玩家得分
	Rank     int    // 排名，从 1 开始
}

// AddPlayerScore 写入/更新玩家得分。
// 基于 Redis ZADD 命令，key 为 "game:rank:global"。
// ZSet 特性：
//   - 自动排序：每次写入后 ZSet 自动按分数排序，无需手动维护
//   - 去重：同一个 playerID 多次写入会覆盖旧分数，玩家只占一个元素
//   - O(logN) 复杂度：ZADD 和查询操作都是 O(logN) 级别，性能优秀
func AddPlayerScore(playerID uint64, score int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ZADD key score member：向有序集合添加成员，若 member 已存在则更新分数
	err := db.GlobalRedis.ZAdd(ctx, globalRankKey, redis.Z{
		Score:  float64(score),
		Member: strconv.FormatUint(playerID, 10),
	}).Err()
	if err != nil {
		logger.Error("排行榜写入失败",
			"player_id", playerID,
			"score", score,
			"error", err.Error(),
		)
		return err
	}

	logger.Info("排行榜写入成功",
		"player_id", playerID,
		"score", score,
	)
	return nil
}

// GetTopN 查询排行榜前 N 名，按分数从高到低排序，排名从 1 开始。
// 基于 Redis ZREVRANGE 命令（分数从高到低）。
// 如果排行榜为空，返回空切片。
func GetTopN(n int) ([]RankItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ZREVRANGE key start stop WITHSCORES：按分数从高到低取出指定范围的成员及分数
	// 索引 0 为最高分，n-1 为第 n 名
	res, err := db.GlobalRedis.ZRevRangeWithScores(ctx, globalRankKey, 0, int64(n-1)).Result()
	if err != nil {
		logger.Error("排行榜 TopN 查询失败",
			"n", n,
			"error", err.Error(),
		)
		return nil, err
	}

	items := make([]RankItem, 0, len(res))
	for i, z := range res {
		playerID, parseErr := strconv.ParseUint(z.Member.(string), 10, 64)
		if parseErr != nil {
			logger.Warn("排行榜成员解析失败",
				"member", z.Member,
				"error", parseErr.Error(),
			)
			continue
		}
		items = append(items, RankItem{
			PlayerID: playerID,
			Score:    int(z.Score),
			Rank:     i + 1, // 排名从 1 开始
		})
	}

	return items, nil
}

// GetPlayerRank 查询单个玩家的排名与得分。
// 使用 ZRANK 获取排名（从低到高），ZREVRANK 获取从高到低的排名，ZSCORE 获取分数。
// 玩家不存在时返回 ErrPlayerNotFound 错误。
func GetPlayerRank(playerID uint64) (rank int, score int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	member := strconv.FormatUint(playerID, 10)

	// ZREVRANK key member：返回成员在 ZSet 中的排名（从高到低，0 为最高分）
	// 成员不存在时返回 redis.Nil
	rankVal, err := db.GlobalRedis.ZRevRank(ctx, globalRankKey, member).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, 0, ErrPlayerNotFound
		}
		logger.Error("排行榜排名查询失败",
			"player_id", playerID,
			"error", err.Error(),
		)
		return 0, 0, err
	}

	// ZSCORE key member：返回成员的分数，成员不存在时返回 redis.Nil
	scoreVal, err := db.GlobalRedis.ZScore(ctx, globalRankKey, member).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, 0, ErrPlayerNotFound
		}
		logger.Error("排行榜分数查询失败",
			"player_id", playerID,
			"error", err.Error(),
		)
		return 0, 0, err
	}

	// ZREVRANK 返回的排名从 0 开始，转换为从 1 开始
	return int(rankVal) + 1, int(scoreVal), nil
}
