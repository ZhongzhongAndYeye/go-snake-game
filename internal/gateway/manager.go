package gateway

import (
	"sync"
	"sync/atomic"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// 全局单例相关变量
var (
	// globalManager 全局 SessionManager 实例，包内所有代码共用
	globalManager *SessionManager

	// globalManagerOnce 确保 GetManager 只初始化一次
	globalManagerOnce sync.Once
)

// SessionManager 全局会话连接管理器。
// 负责管理所有在线玩家的 WebSocket 会话，提供会话的添加、删除、
// 查询和遍历能力。所有方法都是并发安全的，无需外部加锁。
type SessionManager struct {
	// sessions 存储所有在线会话。
	sessions sync.Map

	// nextID 自增会话 ID 生成器，从 1 开始。
	// atomic.Uint64 保证：
	//   - Add(1) 是原子操作，多个 goroutine 同时调用不会重复
	//   - 不需要加锁就能安全地生成唯一 ID
	//   - 服务重启后重置（不持久化，重启后重新从 1 开始）
	nextID atomic.Uint64
}

// GetManager 获取全局单例 SessionManager。
//
// 使用 sync.Once 实现懒加载单例模式：
//   - 第一次调用时创建实例
//   - 后续调用直接返回已创建的实例
//   - 并发安全，无需额外同步
func GetManager() *SessionManager {
	globalManagerOnce.Do(func() {
		globalManager = &SessionManager{}
	}) // do内只会调用一次，就省去了初始化函数
	return globalManager
}

// AddSession 添加一个新会话到管理器，返回自动分配的会话 ID。
// 执行流程：
//  1. 原子自增生成唯一会话 ID（从 1 开始，每次 +1）
//  2. 将会话以 (sessionID → *Session) 的键值对存入 sync.Map
//  3. 打印 Info 日志，记录新会话的 ID、客户端地址、当前在线人数
func (m *SessionManager) AddSession(s *Session) uint64 {
	// 原子自增生成会话 ID（从 1 开始）
	sessionID := m.nextID.Add(1)

	// 存入 sync.Map，key 是 uint64 类型的 sessionID
	m.sessions.Store(sessionID, s)

	// 打印添加日志，记录关键信息便于排查
	logger.Info("会话添加",
		"session_id", sessionID, // 新分配的会话 ID
		"remote", s.RemoteAddr(), // 客户端 IP:Port
		"online", m.Count(), // 当前在线人数
	)
	return sessionID
}

// RemoveSession 按会话 ID 从管理器中移除会话。
// 通常在以下场景调用：
//   - 客户端主动断开连接
//   - 心跳超时被踢下线
//   - 服务端主动关闭（如踢号、封号）
// 注意：RemoveSession 只会从管理器中移除记录，不会关闭会话本身。
// 调用方需要先调用 session.Stop() 关闭连接和 goroutine。
func (m *SessionManager) RemoveSession(sessionID uint64) {
	// 从 sync.Map 中删除
	m.sessions.Delete(sessionID)

	// 打印移除日志
	logger.Info("会话移除",
		"session_id", sessionID,
		"online", m.Count(), // 移除后剩余在线人数
	)
}

// GetSession 按会话 ID 查询会话。
// sync.Map.Load 是 O(1) 复杂度的查询操作，并发安全。
// 如果会话不存在（已移除或从未添加），返回 nil。
func (m *SessionManager) GetSession(sessionID uint64) *Session {
	// Load 返回 (value, ok)，ok 为 false 表示 key 不存在
	val, ok := m.sessions.Load(sessionID)
	if !ok {
		return nil // 会话不存在，返回 nil
	}
	// 类型断言：sync.Map 存储的是 interface{}，需要转回 *Session
	return val.(*Session)
}

// Count 返回当前在线会话数量。
// 通过 Range 遍历整个 sync.Map 计数，时间复杂度 O(n)。
// 当在线人数很多（数万）时，频繁调用可能影响性能，
// 建议只在日志打印和监控上报时使用。
func (m *SessionManager) Count() int {
	count := 0
	// Range 遍历 sync.Map 中的所有键值对
	m.sessions.Range(func(_, _ interface{}) bool {
		count++     // 每遍历到一个就 +1
		return true // 返回 true 继续遍历
	})
	return count
}

// Range 遍历所有在线会话，使用自定义函数操作。
func (m *SessionManager) Range(fn func(sessionID uint64, s *Session) bool) {
	// 代理到 sync.Map.Range，做类型转换：函数返回结果为true，继续，函数结果为false 停止遍历
	m.sessions.Range(func(key, val interface{}) bool {
		return fn(key.(uint64), val.(*Session))
	})
}

// Broadcast 向所有在线会话广播一条消息。
// 通过 Range 遍历所有会话，对每个会话调用 Send 方法发送消息。
// Send 是非阻塞的，如果某个会话的写通道满了会自动丢弃该消息，
// 不会影响其他会话的广播。

func (m *SessionManager) Broadcast(pkt *network.Packet) {
	m.sessions.Range(func(_, val interface{}) bool {
		s := val.(*Session)
		s.Send(pkt) // 非阻塞发送
		return true // 继续广播下一个会话
	})
}
