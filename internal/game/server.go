// 游戏服 gRPC 服务端实现
// 包含匹配、取消匹配、房间信息查询三个 RPC 接口

package game

import (
	"context"
	"errors"

	"go-snake-game/pkg/logger"
	pb "go-snake-game/pkg/proto/rpc"
)

// 业务错误码，统一封装响应中的 code 字段
const (
	CodeSuccess         = 0 // 成功
	CodeInvalidParam    = 1 // 参数格式错误
	CodeMatchFailed     = 2 // 匹配失败
	CodeCancelMatchFail = 3 // 取消匹配失败
	CodeRoomNotFound    = 4 // 房间不存在
)

// GameServerImpl 游戏 gRPC 服务端实现。
// 嵌入 UnimplementedGameServiceServer 保证向前兼容，
// 持有匹配管理器和房间管理器实例完成实际业务逻辑。
type GameServerImpl struct {
	pb.UnimplementedGameServiceServer
	matchManager *MatchManager
	roomManager  *RoomManager
}

// NewGameServer 创建游戏 gRPC 服务端实例。
// 注入匹配管理器与房间管理器全局单例。
func NewGameServer() *GameServerImpl {
	return &GameServerImpl{
		matchManager: GetMatchManager(),
		roomManager:  GetRoomManager(),
	}
}

// StartMatch 发起匹配。
func (s *GameServerImpl) StartMatch(ctx context.Context, req *pb.StartMatchRequest) (*pb.StartMatchResponse, error) {
	playerID := req.GetPlayerId()
	nickname := req.GetNickname()

	logger.Info("gRPC StartMatch", "player_id", playerID, "nickname", nickname)

	// 参数校验
	if playerID == 0 {
		logger.Warn("gRPC StartMatch 参数无效", "player_id", playerID)
		return &pb.StartMatchResponse{Code: CodeInvalidParam, Msg: "玩家 ID 不能为空"}, nil
	}

	// 调用匹配管理器加入队列
	roomID, isMatched, waitingPlayerID, waitingNickname, err := s.matchManager.AddToMatchQueue(playerID, nickname)
	if err != nil {
		logger.Warn("gRPC StartMatch 匹配失败", "player_id", playerID, "error", err.Error())
		return &pb.StartMatchResponse{Code: CodeMatchFailed, Msg: "匹配失败，请稍后重试"}, nil
	}

	if isMatched {
		// 匹配成功，创建房间并将两名玩家加入房间
		room := s.roomManager.CreateRoom(roomID)
		_ = room.AddPlayer(waitingPlayerID, waitingNickname)
		_ = room.AddPlayer(playerID, nickname)

		logger.Info("gRPC StartMatch 匹配成功", "player_id", playerID, "room_id", roomID)
		return &pb.StartMatchResponse{
			Code:      CodeSuccess,
			Msg:       "匹配成功",
			RoomId:    roomID,
			IsMatched: true,
		}, nil
	}

	// 匹配等待中
	logger.Info("gRPC StartMatch 进入等待", "player_id", playerID)
	return &pb.StartMatchResponse{
		Code:      CodeSuccess,
		Msg:       "已进入匹配队列",
		RoomId:    "",
		IsMatched: false,
	}, nil
}

// CancelMatch 取消匹配。
func (s *GameServerImpl) CancelMatch(ctx context.Context, req *pb.CancelMatchRequest) (*pb.CancelMatchResponse, error) {
	playerID := req.GetPlayerId()

	logger.Info("gRPC CancelMatch", "player_id", playerID)

	// 参数校验
	if playerID == 0 {
		logger.Warn("gRPC CancelMatch 参数无效", "player_id", playerID)
		return &pb.CancelMatchResponse{Code: CodeInvalidParam, Msg: "玩家 ID 不能为空"}, nil
	}

	// 调用匹配管理器移除玩家
	err := s.matchManager.RemoveFromMatchQueue(playerID)
	if err != nil {
		if errors.Is(err, ErrPlayerNotInQueue) {
			logger.Warn("gRPC CancelMatch 玩家不在队列", "player_id", playerID)
			return &pb.CancelMatchResponse{Code: CodeCancelMatchFail, Msg: "您不在匹配队列中"}, nil
		}
		logger.Warn("gRPC CancelMatch 取消失败", "player_id", playerID, "error", err.Error())
		return &pb.CancelMatchResponse{Code: CodeCancelMatchFail, Msg: "取消匹配失败，请稍后重试"}, nil
	}

	logger.Info("gRPC CancelMatch 成功", "player_id", playerID)
	return &pb.CancelMatchResponse{Code: CodeSuccess, Msg: "取消匹配成功"}, nil
}

// GetRoomInfo 获取当前房间信息。
func (s *GameServerImpl) GetRoomInfo(ctx context.Context, req *pb.GetRoomInfoRequest) (*pb.GetRoomInfoResponse, error) {
	roomID := req.GetRoomId()

	logger.Info("gRPC GetRoomInfo", "room_id", roomID)

	// 参数校验
	if roomID == "" {
		logger.Warn("gRPC GetRoomInfo 参数无效", "room_id", roomID)
		return &pb.GetRoomInfoResponse{Code: CodeInvalidParam, Msg: "房间 ID 不能为空"}, nil
	}

	// 查询房间管理器
	room, ok := s.roomManager.GetRoom(roomID)
	if !ok {
		logger.Warn("gRPC GetRoomInfo 房间不存在", "room_id", roomID)
		return &pb.GetRoomInfoResponse{Code: CodeRoomNotFound, Msg: "房间不存在"}, nil
	}

	// 转换玩家列表
	players := make([]*pb.PlayerInfo, 0, len(room.Players))
	for _, p := range room.Players {
		players = append(players, &pb.PlayerInfo{
			PlayerId: p.PlayerID,
			Nickname: p.Nickname,
			Score:    p.Score,
		})
	}

	logger.Info("gRPC GetRoomInfo 成功", "room_id", roomID, "player_count", len(players))
	return &pb.GetRoomInfoResponse{
		Code:    CodeSuccess,
		Msg:     "获取房间信息成功",
		RoomId:  roomID,
		Players: players,
		Status:  room.Status,
	}, nil
}
