package gateway

import (
	"sync"
	"sync/atomic"
	"time"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// 【session 管理器】

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

	// heartbeatTimeout 心跳超时时间，超过此时间未收到任何消息的会话将被断开。
	// 默认 60 秒，可通过 SetHeartbeatTimeout 修改。
	heartbeatTimeout time.Duration

	// heartbeatStopCh 心跳检测扫描协程的停止信号通道。
	// 调用 StopHeartbeatCheck 时关闭此通道，通知扫描协程退出。
	heartbeatStopCh chan struct{}
}

// GetManager 获取全局单例 SessionManager。
//
// 使用 sync.Once 实现懒加载单例模式：
//   - 第一次调用时创建实例，初始化心跳超时默认值，并启动心跳检测协程
//   - 后续调用直接返回已创建的实例
//   - 并发安全，无需额外同步
func GetManager() *SessionManager {
	globalManagerOnce.Do(func() {
		globalManager = &SessionManager{
			heartbeatTimeout: 60 * time.Second, // 默认心跳超时 60 秒
			heartbeatStopCh:  make(chan struct{}),
		}
		globalManager.StartHeartbeatCheck() // 默认启动心跳检测
	})
	return globalManager
}

// AddSession 添加一个新会话到管理器，返回自动分配的会话 ID。
// 执行流程：
//  1. 原子自增生成唯一会话 ID（从 1 开始，每次 +1）
//  2. 将会话以 (sessionID → *Session) 的键值对存入 sync.Map
//  3. 在会话上记录 sessionID，供 Stop() 移除自身时使用
//  4. 打印 Info 日志，记录新会话的 ID、客户端地址、当前在线人数
func (m *SessionManager) AddSession(s *Session) uint64 {
	// 原子自增生成会话 ID（从 1 开始）
	sessionID := m.nextID.Add(1)

	// 在会话上记录 sessionID，供 Stop() 移除自身时使用
	s.sessionID = sessionID

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
//
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

// SetHeartbeatTimeout 设置心跳超时时间。
// 超过此时间未收到任何消息的会话将被主动断开。
// 默认为 60 秒。
func (m *SessionManager) SetHeartbeatTimeout(timeout time.Duration) {
	if timeout > 0 {
		m.heartbeatTimeout = timeout
		logger.Info("设置心跳超时", "timeout", timeout.String())
	}
}

// StartHeartbeatCheck 启动心跳检测后台协程。
// 每 10 秒扫描一次所有在线会话，将超过 heartbeatTimeout 未收到消息的会话断开。
// 遍历过程中调用 session.Stop 是并发安全的，不会影响其他会话的正常读写。
func (m *SessionManager) StartHeartbeatCheck() {
	logger.Info("启动心跳检测",
		"timeout", m.heartbeatTimeout.String(),
		"interval", "10s",
	)

	go func() {
		ticker := time.NewTicker(10 * time.Second) // 设置每 10 秒扫描一次
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.checkHeartbeat()
			case <-m.heartbeatStopCh:
				logger.Info("心跳检测已停止")
				return
			}
		}
	}()
}

// StopHeartbeatCheck 优雅停止心跳检测扫描协程。
// 关闭 heartbeatStopCh 通道，通知扫描协程退出。
func (m *SessionManager) StopHeartbeatCheck() {
	select {
	case <-m.heartbeatStopCh:
		// 走到这个分支，说明通道已关闭，避免重复关闭 panic，什么都不做即可
	default:
		// 若是走到这个分支，说明heartbeatStopCh通道未关闭，关闭即可
		close(m.heartbeatStopCh)
	}
}

// checkHeartbeat 执行一次心跳超时扫描。
// 遍历所有会话，检查每个会话的最后心跳时间是否超过阈值。
// 超时的会话调用 Stop 主动断开连接。
func (m *SessionManager) checkHeartbeat() {
	now := time.Now()
	timeout := m.heartbeatTimeout

	// m.sessions 是 sync.Map 类型， Range 是其内置方法，用于 并发安全地遍历 map 中的所有键值对 。
	// 返回 true 继续遍历，返回 false 停止遍历
	// 为什么不使用普通range：防止协程并发操作时改变 map 结构，导致遍历异常
	m.sessions.Range(func(key, val interface{}) bool {
		sessionID := key.(uint64)
		s := val.(*Session)

		// 计算距离上次心跳的时间
		elapsed := now.Sub(s.lastHeartbeat)

		if elapsed > timeout {
			// 心跳超时，主动断开连接
			logger.Warn("心跳超时，断开连接",
				"session_id", sessionID,
				"player_id", s.playerID,
				"remote", s.RemoteAddr(),
				"elapsed", elapsed.String(),
				"timeout", timeout.String(),
			)
			// Stop() 内部会关闭连接、关闭通道、标记离线，并自动从管理器移除
			s.Stop()
		}

		return true // 继续遍历下一个会话
	})
}
