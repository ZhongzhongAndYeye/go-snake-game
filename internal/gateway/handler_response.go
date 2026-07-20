package gateway

import (
	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"google.golang.org/protobuf/proto"
)

// SendSuccess 发送成功响应，自动填充 code=0, msg="ok"。
// msgID 为响应消息 ID，seqID 为请求序列号，body 为已序列化的业务数据。
// 适用于不用额外业务数据的简单响应（如心跳、取消匹配）。
func (s *Session) SendSuccess(msgID uint16, seqID uint16) {
	respBody, _ := proto.Marshal(&msg.ErrorResp{Code: errcode.OK, Msg: "ok"})
	s.Send(&network.Packet{MsgID: msgID, SeqID: seqID, Body: respBody})
}

// SendProtoResponse 发送 proto 序列化后的响应消息。
// 自动处理序列化，失败时记录日志。
// msgID 为响应消息 ID，seqID 为请求序列号，m 为待序列化的 proto 消息。
func (s *Session) SendProtoResponse(msgID uint16, seqID uint16, m proto.Message) {
	body, err := proto.Marshal(m)
	if err != nil {
		logger.Error("响应序列化失败", "msg_id", msgID, "error", err)
		return
	}
	s.Send(&network.Packet{MsgID: msgID, SeqID: seqID, Body: body})
}

// SendErrorResponse 发送统一格式的错误响应，使用 proto ErrorResp 消息。
// 相比 SendError（旧版二进制格式），此方法基于 proto 序列化，客户端解析逻辑统一。
// code 为错误码，errMsg 为错误提示信息。
func (s *Session) SendErrorResponse(seqID uint16, code uint16, errMsg string) {
	s.SendProtoResponse(network.MsgIDErrorResp, seqID, &msg.ErrorResp{
		Code: int32(code),
		Msg:  errMsg,
	})
}
