// Package handler 提供消息处理函数类型定义和 Session 接口，避免子包间循环依赖。
package handler

import (
	"fmt"

	"go-snake-game/pkg/errcode"
)

// 本地错误码常量（仅网关内部使用，未归入全局 errcode 包的错误）。
const (
	ErrCodeMsgNotFound uint16 = 3 // 消息不存在，客户端发送了未注册的消息 ID
)

// ErrorCodeName 返回错误码对应的英文名称，用于日志记录。
func ErrorCodeName(code uint16) string {
	switch code {
	case errcode.OK:
		return "SUCCESS"
	case errcode.ErrParam:
		return "PARAM_ERROR"
	case errcode.ErrSystem:
		return "SYSTEM_ERROR"
	case errcode.ErrNotLogin:
		return "NOT_LOGIN"
	case ErrCodeMsgNotFound:
		return "MSG_NOT_FOUND"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", code)
	}
}
