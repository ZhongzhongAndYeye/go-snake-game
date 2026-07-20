package gateway

import (
	"context"
	"net/http"
	"time"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"

	"github.com/gorilla/websocket"
)

// 【网关服务器启动模块】

// GatewayServer WebSocket 网关服务器。
type GatewayServer struct {
	listenAddr string              // listenAddr 监听地址，格式如 ":8080"
	upgrader   *websocket.Upgrader // WebSocket升级器，将 HTTP 请求升级为 WebSocket 连接
	httpServer *http.Server        // HTTP 服务实例，用于启动和优雅关闭
	router     *Router             // 消息路由器，用于分发客户端消息到对应的处理函数
}

// NewGatewayServer 创建网关服务器实例。
// 参数：
//   - listenAddr: 监听地址，格式如 ":8080"
//
// 返回值：
//   - *GatewayServer: 新创建的网关服务器实例
//
// 内部配置说明：
//   - ReadBufferSize: 读缓冲区 4KB，用于暂存客户端发来的数据
//   - WriteBufferSize: 写缓冲区 4KB，用于暂存要发送给客户端的数据
//   - CheckOrigin: 返回 true 表示允许所有跨域请求
//     （生产环境建议根据实际域名配置白名单）
func NewGatewayServer(listenAddr string) *GatewayServer {
	// 创建消息路由器（自动挂载日志中间件）
	router := NewRouter()

	// 注册免鉴权消息处理函数（不经过 AuthMiddleware）
	router.RegisterPublic(network.MsgIDHeartbeatReq, HeartbeatHandler) // 1001：客户端定期发送，保持连接
	router.RegisterPublic(network.MsgIDRegisterReq, RegisterHandler)   // 1006：新用户注册账号
	router.RegisterPublic(network.MsgIDLoginReq, LoginHandler)         // 1003：用户登录获取 Token

	// 注册鉴权中间件，后续 Register 的消息都会经过鉴权
	router.Use(AuthMiddleware)

	// 注册游戏业务消息（需鉴权，经过 AuthMiddleware）
	router.Register(network.MsgIDMatchStartReq, MatchStartHandler)       // 2001：发起匹配
	router.Register(network.MsgIDMatchCancelReq, MatchCancelHandler)     // 2003：取消匹配
	router.Register(network.MsgIDGameOperationReq, GameOperationHandler) // 3001：游戏操作
	router.Register(network.MsgIDGameRoomInfoReq, RoomInfoQueryHandler)  // 2006：查询房间信息

	return &GatewayServer{
		listenAddr: listenAddr,
		router:     router,
		// 配置 WebSocket 升级器
		upgrader: &websocket.Upgrader{
			// 读缓冲区大小：4096 字节（4KB）
			// 客户端发来的数据会先存到这个缓冲区，然后被 readLoop 读取
			ReadBufferSize: 4096,

			// 写缓冲区大小：4096 字节（4KB）
			// 要发送给客户端的数据会先存到这个缓冲区，然后通过网络发送
			WriteBufferSize: 4096,

			// CheckOrigin 用于跨域检查
			// 返回 true 表示允许任何来源的请求建立连接
			// 开发环境可以直接返回 true，生产环境建议检查请求的 Origin 头
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Start 启动网关服务器。
// 创建 HTTP 多路复用器（mux），注册 /ws 路由，然后启动 HTTP 服务。
// 该方法会阻塞当前 goroutine，直到服务器关闭或出错。
// 使用示例：
//
//	server := NewGatewayServer(":8080")
//	if err := server.Start(); err != nil {
//	    logger.Fatal("启动网关失败", "error", err)
//	}
func (s *GatewayServer) Start() error {
	// 创建 HTTP 多路复用器（路由器）
	// 用于将不同路径的请求分发到不同的处理函数
	mux := http.NewServeMux()

	// 注册 /ws 路由，指定处理函数为 handleConnection
	// 当客户端请求 ws://localhost:8080/ws 时，会调用 handleConnection
	// 本质是给每个连进来的客户端创建一个 goroutine 来处理连接请求
	mux.HandleFunc("/ws", s.handleConnection)

	// 创建 HTTP 服务实例
	// Addr: 监听地址
	// Handler: 请求处理器（这里是 mux）
	s.httpServer = &http.Server{
		Addr:    s.listenAddr,
		Handler: mux,
	}

	// 打印启动日志
	logger.Info("网关服务器启动",
		"listen_addr", s.listenAddr,
	)

	// 启动 HTTP 服务，阻塞当前 goroutine
	// ListenAndServe 会一直运行，直到收到 Shutdown 信号或发生错误
	return s.httpServer.ListenAndServe()
}

// Stop 优雅关闭网关服务器。
// 使用 http.Server.Shutdown 实现优雅关闭：
//  1. 停止接受新的连接
//  2. 等待已有的连接处理完当前请求
//  3. 最多等待 5 秒，超时后强制关闭
//
// 使用示例：
//
//	// 收到 SIGINT/SIGTERM 信号时调用
//	server.Stop()
func (s *GatewayServer) Stop() error {
	// 如果 httpServer 还没创建，直接返回
	if s.httpServer == nil {
		return nil
	}

	// 打印关闭日志
	logger.Info("网关服务器正在关闭",
		"listen_addr", s.listenAddr,
	)

	// 创建一个带超时的 context，最多等待 5 秒
	// 如果 5 秒内还有连接没关闭，Shutdown 会返回超时错误
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // 函数结束时释放资源

	// 调用 Shutdown 优雅关闭
	// Shutdown 会：
	//   1. 停止接受新的 HTTP 请求
	//   2. 通知所有已连接的客户端关闭连接
	//   3. 等待所有连接关闭
	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		// 关闭失败，打印错误日志
		logger.Error("网关服务器关闭失败",
			"error", err.Error(),
		)
		return err
	}

	// 关闭成功
	logger.Info("网关服务器已关闭",
		"listen_addr", s.listenAddr,
	)

	return nil
}

// handleConnection 处理 WebSocket 连接请求。
//
// 这是 /ws 路由的处理函数，每个新的 WebSocket 连接都会创建一个 goroutine
// 执行流程：
//  1. 将 HTTP 请求升级为 WebSocket 连接
//  2. 创建 Session 管理该连接
//  3. 将 Session 加入全局管理器
//  4. 启动 Session（启动读写 goroutine）
//  5. 阻塞等待会话结束（通过 stopCh）
//  6. 会话结束后，从管理器移除
//
// 参数：
//   - w: HTTP 响应写入器，用于发送响应
//   - r: HTTP 请求，包含客户端信息
func (s *GatewayServer) handleConnection(w http.ResponseWriter, r *http.Request) {
	// 第一步：将 HTTP 请求升级为 WebSocket 连接
	// upgrader.Upgrade 会：
	//   1. 检查请求是否符合 WebSocket 协议
	//   2. 发送 101 Switching Protocols 响应
	//   3. 握手成功后返回 WebSocket 连接对象
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// 升级失败（可能是非法请求或网络问题），记录错误日志
		logger.Error("WebSocket 升级失败",
			"remote", r.RemoteAddr, // 客户端 IP:Port
			"error", err.Error(), // 错误信息
		)
		return
	}

	// 升级成功，记录连接日志
	logger.Info("新客户端连接",
		"remote", conn.RemoteAddr(), // 客户端地址
	)

	// 第二步：创建 Session，封装 WebSocket 连接
	// Session 负责管理消息的读写和心跳维护
	// 将路由器传给 Session，用于消息分发
	session := NewSession(conn, s.router)

	// 第三步：将 Session 加入全局管理器
	// 管理器会分配一个唯一的会话 ID，并保存到 sync.Map 中
	sessionID := GetManager().AddSession(session)

	// 第四步：启动 Session
	// 启动读 goroutine 和写 goroutine，开始处理消息收发
	session.Start()

	// 第五步：阻塞等待会话结束
	// stopCh 是一个空结构体通道，关闭时会通知所有等待的 goroutine
	// 当 Session.Stop() 被调用时，stopCh 会被关闭，这里会解除阻塞
	<-session.stopCh

	// 记录连接断开日志
	logger.Info("客户端连接断开",
		"remote", conn.RemoteAddr(), // 客户端地址
		"session_id", sessionID, // 会话 ID
	)
}
