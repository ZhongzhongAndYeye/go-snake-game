// Package gateway 提供 WebSocket 网关服务器，负责客户端连接管理、消息路由分发和 gRPC 回调注入。
package gateway

import (
	"context"
	"net/http"
	"time"

	"go-snake-game/internal/gateway/handler"
	"go-snake-game/internal/gateway/middleware"
	"go-snake-game/internal/gateway/rpc"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"

	"github.com/gorilla/websocket"
)

// GatewayServer WebSocket 网关服务器。
type GatewayServer struct {
	listenAddr string
	upgrader   *websocket.Upgrader
	httpServer *http.Server
	router     *Router
}

// NewGatewayServer 创建网关服务器实例。
func NewGatewayServer(listenAddr string) *GatewayServer {
	router := NewRouter()

	// 注册免鉴权消息处理函数
	router.RegisterPublic(network.MsgIDHeartbeatReq, handler.HeartbeatHandler)
	router.RegisterPublic(network.MsgIDRegisterReq, handler.RegisterHandler)
	router.RegisterPublic(network.MsgIDLoginReq, handler.LoginHandler)

	// 注册限流中间件
	router.Use(middleware.RateLimitMiddleware)

	// 注册鉴权中间件
	router.Use(middleware.AuthMiddleware)

	// 注册游戏业务消息（需鉴权）
	router.Register(network.MsgIDMatchStartReq, handler.MatchStartHandler)
	router.Register(network.MsgIDMatchCancelReq, handler.MatchCancelHandler)
	router.Register(network.MsgIDGameOperationReq, handler.GameOperationHandler)
	router.Register(network.MsgIDGameRoomInfoReq, handler.RoomInfoQueryHandler)
	router.Register(network.MsgIDRankQueryReq, handler.RankQueryHandler)
	router.RegisterPublic(network.MsgIDClearMatchQueueReq, handler.ClearMatchQueueHandler)

	// 注入 rpc 包的 BroadcastToRoom / SendToPlayer 回调
	rpc.BroadcastToRoom = func(roomID string, pkt *network.Packet) int {
		return GetManager().BroadcastToRoom(roomID, pkt)
	}
	rpc.SendToPlayer = func(playerID uint64, pkt *network.Packet) bool {
		session := GetManager().GetSessionByPlayerID(playerID)
		if session == nil || !session.IsOnline() {
			return false
		}
		session.Send(pkt)
		return true
	}

	// 注入 handler 包的 JoinRoomFunc 回调，用于匹配成功时将玩家加入网关房间分组
	handler.JoinRoomFunc = func(playerID uint64, roomID string) {
		GetManager().JoinRoom(playerID, roomID)
	}

	// 注入 rpc 包的 JoinRoom 回调，用于游戏服通过 gRPC 将玩家加入房间分组
	rpc.JoinRoom = func(playerID uint64, roomID string) {
		GetManager().JoinRoom(playerID, roomID)
	}

	return &GatewayServer{
		listenAddr: listenAddr,
		router:     router,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Start 启动网关服务器。
func (s *GatewayServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleConnection)

	s.httpServer = &http.Server{
		Addr:    s.listenAddr,
		Handler: mux,
	}

	logger.Info("网关服务器启动", "listen_addr", s.listenAddr)
	return s.httpServer.ListenAndServe()
}

// Stop 优雅关闭网关服务器。
func (s *GatewayServer) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	logger.Info("网关服务器正在关闭", "listen_addr", s.listenAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		logger.Error("网关服务器关闭失败", "error", err.Error())
		return err
	}

	logger.Info("网关服务器已关闭", "listen_addr", s.listenAddr)
	return nil
}

// handleConnection 处理 WebSocket 连接请求。
func (s *GatewayServer) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("WebSocket 升级失败", "remote", r.RemoteAddr, "error", err.Error())
		return
	}

	logger.Info("新客户端连接", "remote", conn.RemoteAddr())

	session := NewSession(conn, s.router)
	sessionID := GetManager().AddSession(session)
	session.Start()

	<-session.stopCh

	logger.Info("客户端连接断开", "remote", conn.RemoteAddr(), "session_id", sessionID)
}
