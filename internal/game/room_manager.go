// 房间管理器 — 全局单例，管理所有游戏房间

package game

import (
	"sync"
	"time"

	"go-snake-game/pkg/logger"
)

// RoomManager 房间管理器，用 sync.Map 存储所有房间，key 为 roomID。
// sync.Map 适合读多写少的场景，且无需额外加锁，并发安全。
type RoomManager struct {
	rooms        sync.Map  // key: roomID, value: *Room
	playerToRoom sync.Map  // key: playerID (uint64), value: roomID (string)
	cleanupOnce  sync.Once // 确保清理协程只启动一次
}

var (
	roomManagerInstance *RoomManager
	roomManagerOnce     sync.Once
)

// GetRoomManager 获取 RoomManager 全局单例。
// 首次调用时会自动启动后台房间清理协程。
func GetRoomManager() *RoomManager {
	roomManagerOnce.Do(func() {
		roomManagerInstance = &RoomManager{}
		roomManagerInstance.StartCleanupRoutine()
	})
	return roomManagerInstance
}

// CreateRoom 创建新房间并存入管理器。
// 返回创建的 Room 指针。
func (m *RoomManager) CreateRoom(roomID string) *Room {
	room := NewRoom(roomID)
	m.rooms.Store(roomID, room)

	logger.Info("房间创建成功", "room_id", roomID)
	return room
}

// GetRoom 根据 roomID 查询房间。
// 返回房间指针和是否存在。
func (m *RoomManager) GetRoom(roomID string) (*Room, bool) {
	val, ok := m.rooms.Load(roomID)
	if !ok {
		return nil, false
	}
	return val.(*Room), true
}

// RemoveRoom 根据 roomID 删除房间。
func (m *RoomManager) RemoveRoom(roomID string) {
	m.rooms.Delete(roomID)
	logger.Info("房间删除成功", "room_id", roomID)
}

// GetRoomCount 获取当前房间总数。
func (m *RoomManager) GetRoomCount() int {
	count := 0
	m.rooms.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// BindPlayerToRoom 将玩家绑定到房间。
func (m *RoomManager) BindPlayerToRoom(playerID uint64, roomID string) {
	m.playerToRoom.Store(playerID, roomID)
}

// GetPlayerRoom 根据玩家 ID 查询玩家所在的房间 ID。
// 返回房间 ID 和是否存在。
func (m *RoomManager) GetPlayerRoom(playerID uint64) (string, bool) {
	val, ok := m.playerToRoom.Load(playerID)
	if !ok {
		return "", false
	}
	return val.(string), true
}

// UnbindPlayerRoom 解绑玩家与房间的关联。
func (m *RoomManager) UnbindPlayerRoom(playerID uint64) {
	m.playerToRoom.Delete(playerID)
}

// StartCleanupRoutine 启动后台房间清理协程。
// 每 30 秒扫描一次所有房间，清理已结束超过 1 分钟的房间。
// 使用 sync.Once 确保只启动一个协程。
func (m *RoomManager) StartCleanupRoutine() {
	m.cleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			logger.Info("房间清理协程已启动", "interval", "30s")
			for range ticker.C {
				m.cleanupEndedRooms()
			}
		}()
	})
}

// cleanupEndedRooms 清理已结束超过 1 分钟的房间。
// 扫描所有房间，找到已结束且结束时间超过 1 分钟的房间，
// 解绑玩家、释放资源、从管理器中移除。
func (m *RoomManager) cleanupEndedRooms() {
	var roomsToRemove []string
	now := time.Now()

	// 第一步：收集需要清理的房间 ID（不持有锁，避免长时间阻塞）
	m.rooms.Range(func(key, value any) bool {
		room := value.(*Room)
		room.mu.Lock()
		if room.Status == RoomStatusEnded && !room.EndTime.IsZero() && now.Sub(room.EndTime) > 1*time.Minute {
			roomsToRemove = append(roomsToRemove, room.RoomID)
		}
		room.mu.Unlock()
		return true
	})

	if len(roomsToRemove) == 0 {
		return
	}

	// 第二步：清理房间资源
	for _, roomID := range roomsToRemove {
		room, ok := m.GetRoom(roomID)
		if !ok {
			continue
		}

		// 解绑所有玩家
		for _, p := range room.Players {
			m.UnbindPlayerRoom(p.PlayerID)
		}

		// 释放房间资源（停止定时器、关闭通道、清空引用）
		room.Cleanup()

		// 从管理器中移除
		m.RemoveRoom(roomID)
	}

	logger.Info("房间清理完成",
		"cleaned_count", len(roomsToRemove),
		"remaining_rooms", m.GetRoomCount(),
	)
}
