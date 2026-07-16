// 游戏房间定义 — 房间状态、玩家信息、房间操作

package game

import (
	"errors"
	"sync"
	"time"
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

// Room 游戏房间，管理双人匹配的对战状态。
type Room struct {
	mu         sync.Mutex
	RoomID     string        // 房间 ID，全局唯一
	Status     int32         // 房间状态：1 等待中，2 游戏中，3 已结束
	Players    []*PlayerInfo // 房间内玩家列表
	CreateTime time.Time     // 房间创建时间
	StartTime  time.Time     // 游戏开始时间（零值表示未开始）
}

// NewRoom 创建新房间。
func NewRoom(roomID string) *Room {
	return &Room{
		RoomID:     roomID,
		Status:     RoomStatusWaiting,
		Players:    make([]*PlayerInfo, 0, maxRoomPlayers),
		CreateTime: time.Now(),
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

// StartGame 开始游戏，将房间状态修改为游戏中。
// 房间未等待中或玩家不足 2 人时返回错误。
func (r *Room) StartGame() error {
	r.mu.Lock()
	defer r.mu.Unlock()

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

// EndGame 结束游戏，将房间状态修改为已结束。
// 房间未开始游戏时返回错误。
func (r *Room) EndGame() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Status != RoomStatusPlaying {
		return ErrRoomNotStarted
	}

	r.Status = RoomStatusEnded
	return nil
}