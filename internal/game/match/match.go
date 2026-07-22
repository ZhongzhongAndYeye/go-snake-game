// 匹配管理器 — 基于 Redis List 实现双人匹配队列

package match

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"go-snake-game/internal/game/rpc"
	"go-snake-game/pkg/db"
	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

const (
	// matchQueueKey Redis 匹配等待队列 key
	matchQueueKey = "game:match:queue"

	// matchTimeout 匹配超时时间，超过此时间未匹配成功则自动取消
	matchTimeout = 60 * time.Second

	// matchTimeoutScanInterval 匹配超时扫描间隔
	matchTimeoutScanInterval = 10 * time.Second
)

// 预定义错误，供调用方判断匹配队列操作结果
var (
	ErrMatchQueueEmpty  = errors.New("匹配队列为空")
	ErrPlayerNotInQueue = errors.New("玩家不在匹配队列中")
	ErrRedisOpFailed    = errors.New("Redis 操作失败")
	ErrInvalidQueueData = errors.New("匹配队列数据格式错误")
)

// matchQueueItem 匹配队列元素，存储等待匹配的玩家信息。
type matchQueueItem struct {
	PlayerID uint64    `json:"player_id"`
	Nickname string    `json:"nickname"`
	JoinTime time.Time `json:"join_time"` // 入队时间，用于超时判定
}

// MatchManager 匹配管理器，基于 Redis List 实现双人匹配队列。
// 等待玩家进入队列后，匹配管理器自动尝试与后进入的玩家配对。
type MatchManager struct {
	rdb      *redis.Client
	scanOnce sync.Once // 确保超时扫描协程只启动一次
}

var (
	matchManagerInstance *MatchManager
	matchManagerOnce     sync.Once
)

// GetMatchManager 获取 MatchManager 全局单例。
// 首次调用时会自动启动后台匹配超时扫描协程。
func GetMatchManager() *MatchManager {
	matchManagerOnce.Do(func() {
		matchManagerInstance = &MatchManager{
			rdb: db.GlobalRedis,
		}
		matchManagerInstance.StartTimeoutScanner()
	})
	return matchManagerInstance
}

// AddToMatchQueue 将玩家加入匹配队列。
// 逻辑：
//  1. 先尝试从 Redis 等待队列弹出一名等待玩家
//  2. 弹出成功 → 生成唯一房间 ID，返回房间 ID 与匹配成功
//  3. 弹出失败 → 将当前玩家写入 Redis 等待队列，返回等待中状态
func (m *MatchManager) AddToMatchQueue(playerID uint64, nickname string) (roomID string, isMatched bool, waitingPlayerID uint64, waitingNickname string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 尝试从队列头部弹出一名等待玩家
	val, err := m.rdb.LPop(ctx, matchQueueKey).Result()
	if err != nil {
		if err == redis.Nil {
			// 队列为空，将当前玩家入队等待
			item := matchQueueItem{
				PlayerID: playerID,
				Nickname: nickname,
				JoinTime: time.Now(),
			}
			data, marshalErr := json.Marshal(item)
			if marshalErr != nil {
				logger.Error("匹配队列 JSON 序列化失败", "player_id", playerID, "error", marshalErr.Error())
				return "", false, 0, "", fmt.Errorf("%w: %v", ErrInvalidQueueData, marshalErr)
			}

			if pushErr := m.rdb.RPush(ctx, matchQueueKey, data).Err(); pushErr != nil {
				logger.Error("匹配队列入队失败", "player_id", playerID, "error", pushErr.Error())
				return "", false, 0, "", fmt.Errorf("%w: %v", ErrRedisOpFailed, pushErr)
			}

			logger.Info("玩家进入匹配队列", "player_id", playerID, "nickname", nickname)
			return "", false, 0, "", nil
		}

		// Redis 操作异常
		logger.Error("匹配队列弹出失败", "error", err.Error())
		return "", false, 0, "", fmt.Errorf("%w: %v", ErrRedisOpFailed, err)
	}

	// 成功弹出等待玩家，解析其信息
	var waitingPlayer matchQueueItem
	if unmarshalErr := json.Unmarshal([]byte(val), &waitingPlayer); unmarshalErr != nil {
		logger.Error("匹配队列 JSON 反序列化失败", "data", val, "error", unmarshalErr.Error())
		return "", false, 0, "", fmt.Errorf("%w: %v", ErrInvalidQueueData, unmarshalErr)
	}

	// 生成唯一房间 ID
	roomID = generateRoomID()

	logger.Info("玩家匹配成功",
		"room_id", roomID,
		"player1_id", waitingPlayer.PlayerID,
		"player1_nickname", waitingPlayer.Nickname,
		"player2_id", playerID,
		"player2_nickname", nickname,
	)

	return roomID, true, waitingPlayer.PlayerID, waitingPlayer.Nickname, nil
}

// RemoveFromMatchQueue 将指定玩家从匹配队列中移除。
// 用于主动取消匹配或玩家掉线等场景。
// 遍历队列查找匹配的玩家 ID，找到后从队列中移除。
func (m *MatchManager) RemoveFromMatchQueue(playerID uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取队列中所有元素
	vals, err := m.rdb.LRange(ctx, matchQueueKey, 0, -1).Result()
	if err != nil {
		logger.Error("取消匹配获取队列失败", "player_id", playerID, "error", err.Error())
		return fmt.Errorf("%w: %v", ErrRedisOpFailed, err)
	}

	// 遍历查找匹配的玩家
	for _, val := range vals {
		var item matchQueueItem
		if unmarshalErr := json.Unmarshal([]byte(val), &item); unmarshalErr != nil {
			// 数据格式异常，跳过并继续查找
			logger.Warn("取消匹配跳过异常数据", "data", val, "error", unmarshalErr.Error())
			continue
		}

		if item.PlayerID == playerID {
			// 找到匹配的玩家，从队列中移除
			removed, remErr := m.rdb.LRem(ctx, matchQueueKey, 1, val).Result()
			if remErr != nil {
				logger.Error("取消匹配移除失败", "player_id", playerID, "error", remErr.Error())
				return fmt.Errorf("%w: %v", ErrRedisOpFailed, remErr)
			}
			if removed == 0 {
				// 理论上不会发生，但兜底处理
				logger.Warn("取消匹配移除返回 0", "player_id", playerID)
				return ErrPlayerNotInQueue
			}

			logger.Info("玩家取消匹配成功", "player_id", playerID)
			return nil
		}
	}

	// 队列中未找到该玩家
	logger.Warn("取消匹配，玩家不在队列中", "player_id", playerID)
	return ErrPlayerNotInQueue
}

// generateRoomID 生成全局唯一的房间 ID。
// 格式：时间戳（毫秒）+ 6 位随机数，保证全局唯一。
func generateRoomID() string {
	now := time.Now().UnixMilli()
	// 使用 crypto/rand 生成 0-999999 的随机数，密码学安全
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		// 随机数生成失败时回退到纳秒时间戳低 6 位，保证不阻塞
		return fmt.Sprintf("%d%06d", now, time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("%d%06d", now, n.Int64())
}

// StartTimeoutScanner 启动后台匹配超时扫描协程。
// 每 10 秒扫描一次 Redis 匹配队列，检查是否有玩家等待超过 60 秒。
// 超时玩家自动从队列移除，并通过网关推送超时通知。
// 使用 sync.Once 确保只启动一个协程。
func (m *MatchManager) StartTimeoutScanner() {
	m.scanOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(matchTimeoutScanInterval)
			defer ticker.Stop()

			logger.Info("匹配超时扫描协程已启动", "timeout", matchTimeout, "interval", matchTimeoutScanInterval)
			for range ticker.C {
				m.scanTimeoutPlayers()
			}
		}()
	})
}

// scanTimeoutPlayers 扫描匹配队列中的超时玩家。
// 遍历队列中所有元素，将等待超过 matchTimeout 的玩家移出队列，
// 并通过网关推送超时通知。
func (m *MatchManager) scanTimeoutPlayers() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取队列中所有元素
	vals, err := m.rdb.LRange(ctx, matchQueueKey, 0, -1).Result()
	if err != nil {
		logger.Warn("匹配超时扫描获取队列失败", "error", err.Error())
		return
	}

	if len(vals) == 0 {
		return
	}

	now := time.Now()
	var timeoutPlayers []matchQueueItem

	// 检查超时玩家
	for _, val := range vals {
		var item matchQueueItem
		if unmarshalErr := json.Unmarshal([]byte(val), &item); unmarshalErr != nil {
			logger.Warn("匹配超时扫描跳过异常数据", "data", val, "error", unmarshalErr.Error())
			continue
		}

		if !item.JoinTime.IsZero() && now.Sub(item.JoinTime) > matchTimeout {
			timeoutPlayers = append(timeoutPlayers, item)
		}
	}

	// 移除超时玩家并推送通知
	for _, item := range timeoutPlayers {
		// 从队列中移除
		removed, remErr := m.rdb.LRem(ctx, matchQueueKey, 1, valForItem(item)).Result()
		if remErr != nil {
			logger.Warn("匹配超时移除玩家失败", "player_id", item.PlayerID, "error", remErr.Error())
			continue
		}
		if removed == 0 {
			// 玩家可能已被匹配或取消，跳过
			continue
		}

		logger.Info("匹配超时，玩家已从队列移除", "player_id", item.PlayerID, "waited", now.Sub(item.JoinTime))

		// 通过网关推送超时通知
		if rpc.GlobalGatewayClient != nil {
			notify := &msg.MatchCancelResp{
				Code: errcode.ErrMatchTimeout,
				Msg:  "匹配超时，请重新发起匹配",
			}
			body, marshalErr := proto.Marshal(notify)
			if marshalErr != nil {
				logger.Warn("匹配超时通知序列化失败", "player_id", item.PlayerID, "error", marshalErr.Error())
				continue
			}
			rpc.GlobalGatewayClient.SendPlayerMsg(item.PlayerID, network.MsgIDMatchCancelResp, body)
		}
	}

	if len(timeoutPlayers) > 0 {
		logger.Info("匹配超时扫描完成", "timeout_count", len(timeoutPlayers))
	}
}

// valForItem 将 matchQueueItem 序列化为 JSON 字符串，用于 LRem 定位。
func valForItem(item matchQueueItem) string {
	data, _ := json.Marshal(item)
	return string(data)
}