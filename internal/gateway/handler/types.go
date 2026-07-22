// 消息处理函数类型定义与 Session 接口
// handler 包定义核心类型，middleware 和 gateway 包均导入此包，避免循环依赖

package handler

import (
	"time"

	"go-snake-game/pkg/network"

	"google.golang.org/protobuf/proto"
)

// HandlerFunc 消息处理函数类型。
// 接收当前会话和收到的消息包，负责解析包体并执行业务逻辑。
type HandlerFunc func(s Session, packet *network.Packet)

// MiddlewareFunc 中间件函数类型。
// 接收一个 HandlerFunc，返回一个新的 HandlerFunc。
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

// Session 玩家会话接口，定义 handler 和 middleware 所需的方法。
// gateway.Session 实现此接口，避免子包直接依赖 gateway 包。
type Session interface {
	// LogID 返回会话标识字符串，用于日志追踪。
	LogID() string

	// PlayerID 返回玩家 ID，未登录时为 0。
	PlayerID() uint64

	// SetPlayerID 设置玩家 ID（登录成功后调用）。
	SetPlayerID(id uint64)

	// SetLogin 设置登录状态。
	SetLogin(login bool)

	// SendError 向客户端发送统一格式的错误响应。
	SendError(code uint16, errMsg string)

	// SendProtoResponse 发送 proto 序列化后的响应消息。
	SendProtoResponse(msgID uint16, seqID uint16, m proto.Message)

	// SendSuccess 发送成功响应（code=0, msg="ok"）。
	SendSuccess(msgID uint16, seqID uint16)

	// TraceID 返回当前请求的 TraceID。
	TraceID() string

	// SetTraceID 设置当前请求的 TraceID。
	SetTraceID(id string)

	// SetLastHeartbeat 更新最后心跳时间。
	SetLastHeartbeat(t time.Time)
}
