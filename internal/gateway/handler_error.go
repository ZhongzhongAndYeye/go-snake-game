package gateway

import (
	"fmt"

	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"google.golang.org/protobuf/proto"
)

// 【网关服务器错误应答】

// 本地错误码常量（仅网关内部使用，未归入全局 errcode 包的错误）。
const (
	ErrCodeMsgNotFound uint16 = 3 // 消息不存在，客户端发送了未注册的消息 ID
)

// SendError 向客户端发送统一格式的错误响应（proto 序列化）。
// 所有错误响应统一使用 ErrorResp 消息，客户端解析逻辑统一。
//
// 参数：
//   - code: 错误码（引用 errcode 包常量）
//   - errMsg:  错误描述信息，给客户端展示用
//
// 使用示例：
//
//	s.SendError(errcode.ErrNotLogin, "请先登录")
//	s.SendError(errcode.ErrParam, "用户名不能为空")
func (s *Session) SendError(code uint16, errMsg string) {
	body, _ := proto.Marshal(&msg.ErrorResp{Code: int32(code), Msg: errMsg})
	s.Send(&network.Packet{
		MsgID: network.MsgIDErrorResp,
		SeqID: 0,
		Body:  body,
	})
}

// ErrorCodeName 返回错误码对应的英文名称，用于日志记录。
// 方便在日志中快速识别错误类型。
func ErrorCodeName(code uint16) string {
	switch code {
	case errcode.OK:
		return "SUCCESS"
	case errcode.ErrParam:
		return "PARAM_ERROR"
	case errcode.ErrNotLogin:
		return "NOT_LOGIN"
	case errcode.ErrSystem:
		return "SYSTEM_ERROR"
	case ErrCodeMsgNotFound:
		return "MSG_NOT_FOUND"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", code)
	}
}
