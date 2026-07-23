// Package gateway 提供 WebSocket 网关服务器，负责客户端连接管理、消息路由分发和 gRPC 回调注入。
package gateway

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"

	"go-snake-game/internal/gateway/handler"
	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// 【会话结构】

// Session 玩家会话，管理与客户端的 WebSocket 连接。
// 每个连接对应一个 Session，负责消息的读写和心跳维护。
// 客户端成功连接后，默认未登录，调用登录服的相关登录操作函数成功登录后，会话状态再更新为已登录。
type Session struct {
	conn          *websocket.Conn // WebSocket 连接
	playerID      uint64          // 玩家 ID（登录后赋值，未登录为 0）
	sessionID     uint64          // 会话 ID（由 SessionManager 分配）
	RoomID        string          // 玩家当前所在房间 ID，空字符串代表不在房间
	isOnline      bool            // 是否在线
	isLogin       bool            // 是否已登录
	lastHeartbeat time.Time       // 最后心跳时间
	traceID       string          // 当前请求的 TraceID，用于全链路追踪

	readCh  chan *network.Packet // 读通道：接收客户端消息（缓冲 1024）
	writeCh chan *network.Packet // 写通道：发送消息给客户端（缓冲 1024）

	router *Router // 消息路由器，用于分发消息到对应的处理函数

	// sync.Once ：保证一段代码只执行一次，无论被调用多少次。
	closeOnce sync.Once     // 确保 Stop 只执行一次
	writeMu   sync.Mutex    // 写锁：防止conn.WriteMessage正在写，conn.Close把连接关了 conn.WriteMessage的协程panic
	stopCh    chan struct{} // 停止信号，通知读写 goroutine 退出
}

// NewSession 创建新的玩家会话。
// conn 是已建立的 WebSocket 连接。
// router 是消息路由器，用于分发消息到对应的处理函数。
func NewSession(conn *websocket.Conn, router *Router) *Session {
	return &Session{
		conn:          conn,
		playerID:      0,
		isOnline:      true,
		isLogin:       false,
		lastHeartbeat: time.Now(),
		readCh:        make(chan *network.Packet, 1024),
		writeCh:       make(chan *network.Packet, 1024),
		router:        router,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动读写 goroutine，开始处理消息收发。
func (s *Session) Start() {
	logger.Info("会话启动",
		"remote", s.conn.RemoteAddr(),
		"session_id", s.LogID(),
	)

	go s.readLoop()  // 启动读 goroutine
	go s.writeLoop() // 启动写 goroutine
}

// Stop 优雅关闭会话：关闭连接、通道，标记离线，并从管理器中移除自身。
// 安全地多次调用（sync.Once 保证只执行一次）。
// 执行顺序：标记离线 → 通知读写协程退出 → 关闭写通道 → 关闭连接 → 从管理器移除
func (s *Session) Stop() {
	s.closeOnce.Do(func() {
		logger.Info("会话关闭",
			"player_id", s.playerID,
			"session_id", s.LogID(),
		)

		// 1. 标记离线
		s.isOnline = false

		// 2. 发送停止信号，通知读写 goroutine 退出
		close(s.stopCh)

		// 3. 关闭写通道，writeLoop 收到 nil 后退出
		close(s.writeCh)

		// 4. 关闭 WebSocket 连接
		s.writeMu.Lock()
		s.conn.Close()
		s.writeMu.Unlock()

		// 5. 从 SessionManager 中移除自身
		if s.sessionID > 0 {
			GetManager().RemoveSession(s.sessionID)
		}
	})
}

// readLoop 读 goroutine：循环从 WebSocket 读取消息，通过路由器分发处理。
func (s *Session) readLoop() {
	defer func() {
		logger.Debug("读 goroutine 退出", "session_id", s.LogID())
	}()

	for {
		// 读取消息（二进制消息）
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			// 连接关闭或读取异常，停止会话
			logger.Error("读取消息失败",
				"session_id", s.LogID(),
				"error", err.Error(),
			)
			s.Stop()
			return
		}

		// 用 reader 包装数据，通过 Decode 解码
		// 注意：WebSocket 本身已保证消息完整性，不会出现 TCP 粘包半包问题
		// 但这里仍使用 Decode 来解析我们的自定义协议格式
		reader := bufio.NewReader(&wsReader{data: data})
		pkt, err := network.Decode(reader)
		if err != nil {
			logger.Error("解码消息失败",
				"session_id", s.LogID(),
				"error", err.Error(),
			)
			continue
		}

		// 更新心跳时间
		s.lastHeartbeat = time.Now()

		// 通过路由器分发消息到对应的处理函数
		// 只有连接断开才退出读循环，消息处理失败不影响读循环继续运行
		if s.router != nil {
			err = s.router.Handle(s, pkt)
			if err != nil {
				// 路由失败（找不到消息 ID），自动返回错误响应给客户端
				logger.Warn("路由消息失败",
					"session_id", s.LogID(),
					"msg_id", pkt.MsgID,
					"error", err.Error(),
				)
				s.SendError(handler.ErrCodeMsgNotFound, "未知消息类型")
			}
		} else {
			logger.Warn("路由器未初始化，丢弃消息",
				"session_id", s.LogID(),
				"msg_id", pkt.MsgID,
			)
		}

		// 检查停止信号
		select {
		case <-s.stopCh:
			return
		default:
			// 没有停止信号，继续循环接收消息
		}
	}
}

// writeLoop 写 goroutine：循环从 writeCh 取消息，写入 WebSocket 连接。
func (s *Session) writeLoop() {
	defer func() {
		logger.Debug("写 goroutine 退出", "session_id", s.LogID())
	}()

	for {
		select {
		case pkt, ok := <-s.writeCh:
			if !ok {
				// 写通道已关闭，退出
				return
			}

			// 编码消息为二进制
			data, err := network.Encode(pkt.MsgID, pkt.SeqID, pkt.Body)
			if err != nil {
				logger.Error("编码消息失败",
					"session_id", s.LogID(),
					"msg_id", pkt.MsgID,
					"error", err.Error(),
				)
				continue
			}

			// 写入 WebSocket 连接（加写锁，避免与 conn.Close 并发）
			s.writeMu.Lock()
			err = s.conn.WriteMessage(websocket.BinaryMessage, data)
			s.writeMu.Unlock()

			if err != nil {
				logger.Error("写入消息失败",
					"session_id", s.LogID(),
					"msg_id", pkt.MsgID,
					"error", err.Error(),
				)
				s.Stop() // 若是出错了 stop这个会话以及连接（这时候也会关闭写通道）
				return
			}

		case <-s.stopCh:
			// 收到停止信号，退出
			return
		}
	}
}

// Send 将消息放入写通道，发送给客户端。
// 非阻塞，如果通道满则丢弃并记录警告。
// 如果会话已离线则不发送，防止向已关闭的 writeCh 写入导致 panic。
func (s *Session) Send(pkt *network.Packet) {
	if !s.isOnline {
		return
	}
	select {
	case s.writeCh <- pkt:
	default:
		logger.Warn("写通道已满，丢弃消息",
			"player_id", s.playerID,
			"msg_id", pkt.MsgID,
		)
	}
}

// Read 从读通道读取一条客户端消息。
// 非阻塞返回，如果通道为空则返回 nil。
func (s *Session) Read() *network.Packet {
	select {
	case pkt := <-s.readCh:
		return pkt
	default:
		return nil
	}
}

// PlayerID 返回玩家 ID。
func (s *Session) PlayerID() uint64 {
	return s.playerID
}

// SetPlayerID 设置玩家 ID（登录成功后调用）。
func (s *Session) SetPlayerID(id uint64) {
	s.playerID = id
}

// SetLogin 设置登录状态（登录成功后调用）。
func (s *Session) SetLogin(login bool) {
	s.isLogin = login
}

// SetRoomID 设置玩家当前所在房间 ID。
func (s *Session) SetRoomID(roomID string) {
	s.RoomID = roomID
}

// IsOnline 返回是否在线。
func (s *Session) IsOnline() bool {
	return s.isOnline
}

// LastHeartbeat 返回最后心跳时间。
func (s *Session) LastHeartbeat() time.Time {
	return s.lastHeartbeat
}

// RemoteAddr 返回客户端地址。
func (s *Session) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// LogID 生成会话标识，用于日志追踪。
// 包含会话 ID 和客户端地址，便于区分不同连接。
func (s *Session) LogID() string {
	if s.sessionID > 0 {
		return fmt.Sprintf("%d@%s", s.sessionID, s.conn.RemoteAddr().String())
	}
	return s.conn.RemoteAddr().String()
}

// SetTraceID 设置当前请求的 TraceID，用于全链路日志追踪。
func (s *Session) SetTraceID(id string) {
	s.traceID = id
}

// TraceID 返回当前请求的 TraceID。
func (s *Session) TraceID() string {
	return s.traceID
}

// SetLastHeartbeat 更新最后心跳时间。
func (s *Session) SetLastHeartbeat(t time.Time) {
	s.lastHeartbeat = t
}

// SendError 向客户端发送统一格式的错误响应。
func (s *Session) SendError(code uint16, errMsg string) {
	errResp := &msg.ErrorResp{
		Code: int32(code),
		Msg:  errMsg,
	}
	data, err := proto.Marshal(errResp)
	if err != nil {
		logger.Error("序列化错误响应失败",
			"session_id", s.LogID(),
			"error", err.Error(),
		)
		return
	}
	s.Send(&network.Packet{
		MsgID: network.MsgIDErrorResp,
		Body:  data,
	})
}

// SendProtoResponse 发送 proto 序列化后的响应消息。
func (s *Session) SendProtoResponse(msgID uint16, seqID uint16, m proto.Message) {
	data, err := proto.Marshal(m)
	if err != nil {
		logger.Error("序列化响应消息失败",
			"session_id", s.LogID(),
			"msg_id", msgID,
			"error", err.Error(),
		)
		s.SendError(errcode.ErrSystem, "系统错误")
		return
	}
	s.Send(&network.Packet{
		MsgID: msgID,
		SeqID: seqID,
		Body:  data,
	})
}

// SendSuccess 发送成功响应（code=0, msg="ok"）。
func (s *Session) SendSuccess(msgID uint16, seqID uint16) {
	successResp := &msg.ErrorResp{
		Code: errcode.OK,
		Msg:  "ok",
	}
	data, err := proto.Marshal(successResp)
	if err != nil {
		logger.Error("序列化成功响应失败",
			"session_id", s.LogID(),
			"error", err.Error(),
		)
		return
	}
	s.Send(&network.Packet{
		MsgID: msgID,
		SeqID: seqID,
		Body:  data,
	})
}

// wsReader 包装 []byte 实现 io.Reader，用于 Decode 解码。
// 因为 WebSocket 一次读一条完整消息，不需要 bufio.Reader 的缓冲能力。
type wsReader struct {
	data []byte // WebSocket 读到的一条完整消息的二进制数据
	pos  int    // 当前已读取的位置偏移（字节），下次 Read 从这里继续
}

// Read 的作用是： 每次被调用时，从 data 里取一部分数据给调用者，直到取完为止。（Decode时满足ReadFull的要求，是ReadFull功能实现的关键）
// 比如：第 1 批：只收到 4 字节
// wsReader.data = [0x00, 0x00, 0x00, 0x0D]  ← 只有 4 字节
// Decode:
//   ReadFull 要 8 字节
//   → 调用 wsReader.Read(headerBuf[0:8])
//   → 只能拷贝 4 字节 → 返回 4, nil
//   → ReadFull 发现还差 4 字节
//   → 再次调用 wsReader.Read(headerBuf[4:8])
//   → 此时 data 已读完，pos=4 >= len(data)=4
//   → 返回 0, nil
//   → ReadFull 发现：还没读够，但对方返回了 0 字节
//   → 继续循环调用 Read... 直到有新数据

// 第 2 批：剩余 9 字节到达
// wsReader.data = [0x00,0x00,0x00,0x0D, 0x03,0xE9,0x00,0x01, 0x68,0x65,0x6C,0x6C,0x6F]

// 但注意：这里 wsReader 是一次性构造的，不是真正的流式。
// 真实的流式场景中，Read 会阻塞等待直到有数据或连接关闭。

func (r *wsReader) Read(buf []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, nil // 返回 0 而非 io.EOF，避免 Decode 误判为连接断开
	}
	n := copy(buf, r.data[r.pos:])
	r.pos += n
	return n, nil
}
