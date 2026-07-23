// Package room 提供游戏房间管理，包括房间状态定义、玩家加入/移除、房间生命周期控制等。
package room

import (
	"errors"
	"sync"
	"time"

	"go-snake-game/internal/game/engine"
	"go-snake-game/pkg/logger"
)

// 房间状态枚举
const (
	RoomStatusWaiting = 1 // 等待中，等待玩家加入
	RoomStatusPlaying = 2 // 游戏中，双方已开始对战
	RoomStatusEnded   = 3 // 已结束，游戏已结束
)

// 房间最大玩家数（双人匹配）
const maxRoomPlayers = 2

// 预定义错误，供调用方判断房间操作结果
var (
	ErrRoomFull        = errors.New("房间已满，无法加入")
	ErrRoomInGame      = errors.New("房间正在游戏中，无法操作")
	ErrRoomNotStarted  = errors.New("房间未开始游戏")
	ErrRoomAlreadyEnd  = errors.New("房间已结束")
	ErrPlayerNotInRoom = errors.New("玩家不在房间中")
)

// PlayerInfo 房间内玩家信息。
type PlayerInfo struct {
	PlayerID uint64 // 玩家 ID
	Nickname string // 玩家昵称
	Score    int32  // 玩家分数
	IsOnline bool   // 是否在线
}

// Room 游戏房间，管理双人匹配的对战状态和游戏生命周期。
type Room struct {
	mu         sync.Mutex
	RoomID     string        // 房间 ID，全局唯一
	Status     int32         // 房间状态：1 等待中，2 游戏中，3 已结束
	Players    []*PlayerInfo // 房间内玩家列表
	CreateTime time.Time     // 房间创建时间
	StartTime  time.Time     // 游戏开始时间（零值表示未开始）
	EndTime    time.Time     // 游戏结束时间（零值表示未结束），用于定时清理判断

	// 游戏状态（仅在 Status == Playing 时有效）
	GameStatus  int                      // 游戏阶段：1 未开始，2 进行中，3 暂停，4 已结束
	Frame       int64                    // 当前帧序号，从 1 开始递增
	Snakes      map[uint64]*engine.Snake // 房间内所有玩家的蛇实例，key 为 playerID
	CurrentFood *engine.Food             // 当前地图上的食物
	MapWidth    int                      // 地图宽度
	MapHeight   int                      // 地图高度
	ticker      *time.Ticker             // 游戏帧定时器，10 FPS（100ms 一帧）
	stopCh      chan struct{}            // 停止信号通道，关闭后游戏主循环退出
}

// NewRoom 创建新房间。
func NewRoom(roomID string) *Room {
	return &Room{
		RoomID:     roomID,
		Status:     RoomStatusWaiting,
		Players:    make([]*PlayerInfo, 0, maxRoomPlayers),
		CreateTime: time.Now(),
		GameStatus: engine.GameStatusNotStarted,
		MapWidth:   engine.DefaultMapWidth,
		MapHeight:  engine.DefaultMapHeight,
		Snakes:     make(map[uint64]*engine.Snake),
	}
}

// AddPlayer 将玩家加入房间。
// 房间已满（达到 2 人）或房间正在游戏中时返回错误。
func (r *Room) AddPlayer(playerID uint64, nickname string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status == RoomStatusPlaying {
		return ErrRoomInGame
	}

	if r.Status == RoomStatusEnded {
		return ErrRoomAlreadyEnd
	}

	if len(r.Players) >= maxRoomPlayers {
		return ErrRoomFull
	}

	// 检查是否已在房间中
	for _, p := range r.Players {
		if p.PlayerID == playerID {
			return nil
		}
	}

	r.Players = append(r.Players, &PlayerInfo{
		PlayerID: playerID,
		Nickname: nickname,
		Score:    0,
		IsOnline: true,
	})

	return nil
}

// RemovePlayer 将玩家从房间移除。
func (r *Room) RemovePlayer(playerID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, p := range r.Players {
		if p.PlayerID == playerID {
			r.Players = append(r.Players[:i], r.Players[i+1:]...)
			return nil
		}
	}

	return ErrPlayerNotInRoom
}

// setRoomPlayingLocked 将房间状态标记为游戏中。
// 调用方必须持有 r.mu 锁。
func (r *Room) setRoomPlayingLocked() error {
	if r.Status == RoomStatusPlaying {
		return ErrRoomInGame
	}

	if r.Status == RoomStatusEnded {
		return ErrRoomAlreadyEnd
	}

	if len(r.Players) < maxRoomPlayers {
		return errors.New("玩家不足，无法开始游戏")
	}

	r.Status = RoomStatusPlaying
	r.StartTime = time.Now()
	return nil
}

// setRoomEndedLocked 将房间状态标记为已结束。
// 调用方必须持有 r.mu 锁。
func (r *Room) setRoomEndedLocked() error {
	if r.Status != RoomStatusPlaying {
		return ErrRoomNotStarted
	}

	r.Status = RoomStatusEnded
	return nil
}

// Cleanup 释放房间所有资源，确保游戏循环 goroutine 正常退出。
// 调用方无需持有锁，方法内部会自行加锁。
// 调用后房间不应再被使用。
func (r *Room) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 停止帧定时器
	if r.ticker != nil {
		r.ticker.Stop()
		r.ticker = nil
	}

	// 关闭停止通道，确保游戏主循环 goroutine 退出
	select {
	case <-r.stopCh:
		// 通道已关闭，无需重复关闭
	default:
		close(r.stopCh)
	}

	// 释放引用，帮助 GC 回收
	r.Snakes = nil
	r.CurrentFood = nil
	logger.Info("房间资源清理完成", "room_id", r.RoomID)
}

// Lock 加锁，供外部包在需要原子读取多个房间字段时使用。
// 调用方必须随后调用 Unlock 释放锁。
func (r *Room) Lock() {
	r.mu.Lock()
}

// Unlock 释放锁，与 Lock 配对使用。
func (r *Room) Unlock() {
	r.mu.Unlock()
}
