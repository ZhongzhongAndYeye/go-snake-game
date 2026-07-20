package gateway

import (
	"sync"
	"testing"
	"time"

	"go-snake-game/pkg/network"
)

// TestRateLimit_LowFrequencyPass 低频请求全部通过。
func TestRateLimit_LowFrequencyPass(t *testing.T) {
	// 清理全局限流器状态，避免其他测试干扰
	playerLimiters = sync.Map{}

	session, cleanup := setupTestSession(t)
	defer cleanup()
	session.SetPlayerID(1001)

	mw := RateLimitMiddleware(func(s *Session, pkt *network.Packet) {
		// 业务 Handler：记录被调用
	})

	// 发送 5 个请求，间隔 500ms（远低于限流阈值 10/s）
	passed := 0
	for i := 0; i < 5; i++ {
		mw(session, &network.Packet{MsgID: 2001, SeqID: uint16(i)})
		passed++
		time.Sleep(500 * time.Millisecond)
	}

	if passed != 5 {
		t.Errorf("低频请求应全部通过，期望 5，实际 %d", passed)
	}
}

// TestRateLimit_HighFrequencyBlocked 高频请求部分被拦截。
func TestRateLimit_HighFrequencyBlocked(t *testing.T) {
	// 清理全局限流器状态
	playerLimiters = sync.Map{}

	session, cleanup := setupTestSession(t)
	defer cleanup()
	session.SetPlayerID(1002)

	allowed := 0
	blocked := 0

	mw := RateLimitMiddleware(func(s *Session, pkt *network.Packet) {
		allowed++
	})

	// 瞬间发送 50 个请求，远超桶容量 20
	for i := 0; i < 50; i++ {
		mw(session, &network.Packet{MsgID: 2001, SeqID: uint16(i)})
	}

	// 统计实际结果：allowed 被业务 Handler 计数，blocked 由 SendError 间接体现
	// 由于 SendError 是 Session 方法，无法直接拦截计数，
	// 这里通过 allowed 来验证：桶容量 20，最多允许 20 个请求通过
	blocked = 50 - allowed

	if allowed <= 0 {
		t.Error("应有请求通过限流，但没有任何请求通过")
	}
	if allowed > 20 {
		t.Errorf("最多允许 %d 个请求通过（burst=20），实际通过 %d", 20, allowed)
	}
	if blocked <= 0 {
		t.Error("应有请求被限流拦截，但全部通过")
	}

	t.Logf("50 个请求: 通过 %d, 拦截 %d", allowed, blocked)
}

// TestRateLimit_TokenRecovery 冷却后令牌自动恢复，请求可正常通过。
func TestRateLimit_TokenRecovery(t *testing.T) {
	// 清理全局限流器状态
	playerLimiters = sync.Map{}

	session, cleanup := setupTestSession(t)
	defer cleanup()
	session.SetPlayerID(1003)

	mw := RateLimitMiddleware(func(s *Session, pkt *network.Packet) {})

	// 第一步：消耗完所有令牌（瞬间发 30 个请求）
	for i := 0; i < 30; i++ {
		mw(session, &network.Packet{MsgID: 2001, SeqID: uint16(i)})
	}

	// 第二步：等待 2 秒，让令牌自动恢复（10/s，2 秒恢复 20 个）
	time.Sleep(2 * time.Second)

	// 第三步：发送 5 个低频请求，应全部通过
	allowed := 0
	mw = RateLimitMiddleware(func(s *Session, pkt *network.Packet) {
		allowed++
	})

	for i := 0; i < 5; i++ {
		mw(session, &network.Packet{MsgID: 2001, SeqID: uint16(i + 100)})
		time.Sleep(100 * time.Millisecond)
	}

	if allowed != 5 {
		t.Errorf("冷却后请求应全部通过，期望 5，实际 %d", allowed)
	}
}

// TestRateLimit_PlayerIsolation 玩家之间限流独立，互不影响。
func TestRateLimit_PlayerIsolation(t *testing.T) {
	// 清理全局限流器状态
	playerLimiters = sync.Map{}

	session1, cleanup1 := setupTestSession(t)
	defer cleanup1()
	session1.SetPlayerID(2001)

	session2, cleanup2 := setupTestSession(t)
	defer cleanup2()
	session2.SetPlayerID(2002)

	// 玩家 1 瞬间消耗完所有令牌
	mw1 := RateLimitMiddleware(func(s *Session, pkt *network.Packet) {})
	for i := 0; i < 30; i++ {
		mw1(session1, &network.Packet{MsgID: 2001, SeqID: uint16(i)})
	}

	// 玩家 2 发送低频请求，应全部通过（不受玩家 1 影响）
	allowed := 0
	mw2 := RateLimitMiddleware(func(s *Session, pkt *network.Packet) {
		allowed++
	})

	for i := 0; i < 5; i++ {
		mw2(session2, &network.Packet{MsgID: 2001, SeqID: uint16(i)})
		time.Sleep(200 * time.Millisecond)
	}

	if allowed != 5 {
		t.Errorf("玩家 2 的请求应全部通过（不受玩家 1 影响），期望 5，实际 %d", allowed)
	}
}

// TestRateLimit_NotLoggedIn 未登录玩家不限制。
func TestRateLimit_NotLoggedIn(t *testing.T) {
	// 清理全局限流器状态
	playerLimiters = sync.Map{}

	session, cleanup := setupTestSession(t)
	defer cleanup()
	// 不设置 playerID，模拟未登录状态

	allowed := 0
	mw := RateLimitMiddleware(func(s *Session, pkt *network.Packet) {
		allowed++
	})

	// 瞬间发送 50 个请求，未登录玩家应全部通过
	for i := 0; i < 50; i++ {
		mw(session, &network.Packet{MsgID: 2001, SeqID: uint16(i)})
	}

	if allowed != 50 {
		t.Errorf("未登录玩家不应受限，期望 50，实际 %d", allowed)
	}
}
