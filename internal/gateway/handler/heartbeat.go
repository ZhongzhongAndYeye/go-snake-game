package handler

import (
	"time"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// HeartbeatHandler 心跳请求处理器。
func HeartbeatHandler(s Session, packet *network.Packet) {
	s.SetLastHeartbeat(time.Now())

	logger.Debug("收到心跳",
		"session_id", s.LogID(),
		"player_id", s.PlayerID(),
		"seq_id", packet.SeqID,
	)

	s.SendSuccess(network.MsgIDHeartbeatResp, packet.SeqID)
}