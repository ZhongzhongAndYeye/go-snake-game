// 房间管理器 — 全局单例，管理所有游戏房间

package game

import (
	"sync"

	"go-snake-game/pkg/logger"
)

// RoomManager 房间管理器，用 sync.Map 存储所有房间，key 为 roomID。
// sync.Map 适合读多写少的场景，且无需额外加锁，并发安全。
type RoomManager struct {
	rooms       sync.Map // key: roomID, value: *Room
	playerToRoom sync.Map // key: playerID (uint64), value: roomID (string)
}

var (
	roomManagerInstance *RoomManager
	roomManagerOnce     sync.Once
)

// GetRoomManager 获取 RoomManager 全局单例。
func GetRoomManager() *RoomManager {
	roomManagerOnce.Do(func() {
		roomManagerInstance = &RoomManager{}
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
