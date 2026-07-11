package gateway

import (
	"fmt"

	"go-snake-game/pkg/network"
)

// 【网关服务器错误应答】

// ---- 通用错误码常量 ----
const (
	ErrCodeSuccess     uint16 = 0 // 成功，无错误
	ErrCodeParamError  uint16 = 1 // 参数错误，客户端请求参数不合法
	ErrCodeNotLogin    uint16 = 2 // 未登录，玩家未登录时尝试访问需要登录的接口
	ErrCodeMsgNotFound uint16 = 3 // 消息不存在，客户端发送了未注册的消息 ID
	ErrCodeSystemError uint16 = 4 // 系统内部错误，服务端处理异常
)

// SendError 向客户端发送统一格式的错误响应。
//
// 参数：
//   - code: 错误码（见上方常量定义）
//   - msg:  错误描述信息，给客户端展示用
//
// 包体格式（临时用简单字节拼接，后续接入 Protobuf 后替换）：
//
//	低 2 字节：错误码（uint16，大端序）
//	后续字节：错误信息文本（UTF-8 字符串）
//
// 使用示例：
//
//	s.SendError(ErrCodeNotLogin, "请先登录")
//	s.SendError(ErrCodeParamError, "用户名不能为空")
func (s *Session) SendError(code uint16, msg string) {
	// 构建包体：错误码（2字节）+ 错误信息
	body := make([]byte, 2+len(msg))

	// 错误body：前两个字节是错误码code，后续字节是错误信息文本msg
	// 写入错误码（大端序，uint16 → 2字节）
	body[0] = byte(code >> 8)   // 高字节
	body[1] = byte(code & 0xFF) // 低字节

	// 写入错误信息（UTF-8 字符串）
	copy(body[2:], msg)

	// 封装成 Packet 并发送
	s.Send(&network.Packet{
		MsgID: network.MsgIDErrorResp,
		SeqID: 0, // 错误响应不带序列号，SeqID=0
		Body:  body,
	})
}

// ErrorCodeName 返回错误码对应的英文名称，用于日志记录。
// 方便在日志中快速识别错误类型。
func ErrorCodeName(code uint16) string {
	switch code {
	case ErrCodeSuccess:
		return "SUCCESS"
	case ErrCodeParamError:
		return "PARAM_ERROR"
	case ErrCodeNotLogin:
		return "NOT_LOGIN"
	case ErrCodeMsgNotFound:
		return "MSG_NOT_FOUND"
	case ErrCodeSystemError:
		return "SYSTEM_ERROR"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", code)
	}
}
