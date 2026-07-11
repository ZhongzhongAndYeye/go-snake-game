package gateway

import (
	"time"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// HeartbeatHandler 心跳请求处理器。
// 收到客户端的心跳请求后：
//  1. 更新会话的最后心跳时间，用于检测客户端是否离线
//  2. 回复心跳响应，确认连接正常
//  3. 记录心跳日志（Debug级别，生产环境可关闭）
//
// 心跳请求（MsgID=1001）：包体为空
// 心跳响应（MsgID=1002）：包体为空，SeqID与请求一致
func HeartbeatHandler(s *Session, packet *network.Packet) {
	// 更新最后心跳时间，标记客户端在线
	s.lastHeartbeat = time.Now()

	// 记录心跳日志（Debug级别，开发调试时使用）
	logger.Debug("收到心跳",
		"session_id", s.sessionID(),
		"player_id", s.playerID,
		"seq_id", packet.SeqID,
	)

	// 回复心跳响应：MsgID=1002，SeqID与请求一致，包体为空
	s.Send(&network.Packet{
		MsgID: network.MsgIDHeartbeatResp,
		SeqID: packet.SeqID,
		Body:  nil,
	})
}
