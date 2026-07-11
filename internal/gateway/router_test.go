package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-snake-game/pkg/network"

	"github.com/gorilla/websocket"
)

// setupTestSession 创建测试用的真实 WebSocket 连接和 Session。
// 返回 (session, cleanupFunc)，测试结束后调用 cleanupFunc 关闭连接。
func setupTestSession(t *testing.T) (*Session, func()) {
	t.Helper()

	// 创建测试 HTTP 服务器，将 HTTP 连接升级为 WebSocket
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// 保持连接存活，直到客户端关闭
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))

	// 客户端连接测试服务器
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("连接测试服务器失败: %v", err)
	}

	session := NewSession(conn, nil)
	return session, func() {
		conn.Close()
		srv.Close()
	}
}

// TestRouter_RegisterAndHandle 测试注册消息后，Handle 能正确调用对应 Handler。
func TestRouter_RegisterAndHandle(t *testing.T) {
	session, cleanup := setupTestSession(t)
	defer cleanup()

	router := NewRouter()
	var (
		handlerCalled bool
		gotSession    *Session
		gotPacket     *network.Packet
	)

	// 注册一个 Handler
	router.Register(1001, func(s *Session, pkt *network.Packet) {
		handlerCalled = true
		gotSession = s
		gotPacket = pkt
	})

	// 分发消息
	pkt := &network.Packet{MsgID: 1001, SeqID: 1, Body: []byte("hello")}
	err := router.Handle(session, pkt)
	if err != nil {
		t.Fatalf("Handle 返回错误: %v", err)
	}

	// 验证 Handler 被调用
	if !handlerCalled {
		t.Fatal("Handler 未被调用")
	}
	if gotSession != session {
		t.Fatal("Handler 收到的 Session 不正确")
	}
	if gotPacket != pkt {
		t.Fatal("Handler 收到的 Packet 不正确")
	}
}

// TestRouter_UnknownMsgID 测试不存在的消息 ID 返回错误。
func TestRouter_UnknownMsgID(t *testing.T) {
	session, cleanup := setupTestSession(t)
	defer cleanup()

	router := NewRouter()

	// 分发未注册的消息
	pkt := &network.Packet{MsgID: 9999}
	err := router.Handle(session, pkt)

	if err == nil {
		t.Fatal("期望返回错误，但得到 nil")
	}
	if !strings.Contains(err.Error(), "unknown message id") {
		t.Errorf("错误信息不正确: %v", err)
	}
}

// TestRouter_Middleware 测试中间件能正常执行并调用 next。
func TestRouter_Middleware(t *testing.T) {
	session, cleanup := setupTestSession(t)
	defer cleanup()

	router := NewRouter()

	// 记录执行顺序
	var executionOrder []string

	// 添加自定义中间件：记录执行顺序
	router.Use(func(next HandlerFunc) HandlerFunc {
		return func(s *Session, pkt *network.Packet) {
			executionOrder = append(executionOrder, "middleware_before")
			next(s, pkt)
			executionOrder = append(executionOrder, "middleware_after")
		}
	})

	// 注册业务 Handler
	router.Register(2001, func(s *Session, pkt *network.Packet) {
		executionOrder = append(executionOrder, "handler")
	})

	pkt := &network.Packet{MsgID: 2001}
	err := router.Handle(session, pkt)
	if err != nil {
		t.Fatalf("Handle 返回错误: %v", err)
	}

	// 验证执行顺序：
	// LogMiddleware(next) → custom_middleware(before) → handler → custom_middleware(after) → LogMiddleware(defer)
	expected := []string{"middleware_before", "handler", "middleware_after"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("期望 %d 次执行，实际 %d 次: %v", len(expected), len(executionOrder), executionOrder)
	}
	for i, e := range expected {
		if executionOrder[i] != e {
			t.Errorf("第 %d 步: 期望 %s, 实际 %s", i, e, executionOrder[i])
		}
	}
}

// TestRouter_MiddlewareChain 测试多个中间件按洋葱模型正确链式执行。
func TestRouter_MiddlewareChain(t *testing.T) {
	session, cleanup := setupTestSession(t)
	defer cleanup()

	router := NewRouter()

	var executionOrder []string

	// 第一个中间件（最外层）
	router.Use(func(next HandlerFunc) HandlerFunc {
		return func(s *Session, pkt *network.Packet) {
			executionOrder = append(executionOrder, "mw1_before")
			next(s, pkt)
			executionOrder = append(executionOrder, "mw1_after")
		}
	})

	// 第二个中间件（内层，紧贴业务 Handler）
	router.Use(func(next HandlerFunc) HandlerFunc {
		return func(s *Session, pkt *network.Packet) {
			executionOrder = append(executionOrder, "mw2_before")
			next(s, pkt)
			executionOrder = append(executionOrder, "mw2_after")
		}
	})

	router.Register(3001, func(s *Session, pkt *network.Packet) {
		executionOrder = append(executionOrder, "handler")
	})

	pkt := &network.Packet{MsgID: 3001}
	err := router.Handle(session, pkt)
	if err != nil {
		t.Fatalf("Handle 返回错误: %v", err)
	}

	// 执行顺序应为：LogMiddleware → mw1_before → mw2_before → handler → mw2_after → mw1_after
	// 但 LogMiddleware 只做 defer 记录，不记录到 executionOrder
	expected := []string{"mw1_before", "mw2_before", "handler", "mw2_after", "mw1_after"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("期望 %d 次执行，实际 %d 次: %v", len(expected), len(executionOrder), executionOrder)
	}
	for i, e := range expected {
		if executionOrder[i] != e {
			t.Errorf("第 %d 步: 期望 %s, 实际 %s", i, e, executionOrder[i])
		}
	}
}
