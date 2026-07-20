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
	CodeOperationFail   = 5 // 游戏操作失败
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

		// 绑定玩家到房间
		s.roomManager.BindPlayerToRoom(waitingPlayerID, roomID)
		s.roomManager.BindPlayerToRoom(playerID, roomID)

		// 房间人满（2 人），自动开始游戏
		if err := room.StartGame(); err != nil {
			logger.Warn("gRPC StartMatch 开始游戏失败", "room_id", roomID, "error", err.Error())
		}

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

	room.mu.Lock()
	defer room.mu.Unlock()

	// 转换玩家列表
	players := make([]*pb.PlayerInfo, 0, len(room.Players))
	for _, p := range room.Players {
		players = append(players, &pb.PlayerInfo{
			PlayerId: p.PlayerID,
			Nickname: p.Nickname,
			Score:    p.Score,
		})
	}

	// 转换蛇状态
	snakes := make([]*pb.SnakeState, 0, len(room.Snakes))
	for _, snake := range room.Snakes {
		body := make([]*pb.Point, 0, len(snake.Body))
		for _, pt := range snake.Body {
			body = append(body, &pb.Point{X: int32(pt.X), Y: int32(pt.Y)})
		}
		snakes = append(snakes, &pb.SnakeState{
			PlayerId: snake.PlayerID,
			Nickname: snake.Nickname,
			Body:     body,
			Score:    int32(snake.Score),
			IsAlive:  snake.IsAlive,
		})
	}

	// 转换食物状态
	var food *pb.FoodState
	if room.CurrentFood != nil {
		rf := room.CurrentFood
		food = &pb.FoodState{
			Position: &pb.Point{X: int32(rf.Position.X), Y: int32(rf.Position.Y)},
			Score:    int32(rf.ScoreValue),
		}
	}

	logger.Info("gRPC GetRoomInfo 成功", "room_id", roomID, "player_count", len(players), "game_status", room.GameStatus)
	return &pb.GetRoomInfoResponse{
		Code:       CodeSuccess,
		Msg:        "获取房间信息成功",
		RoomId:     roomID,
		Players:    players,
		Status:     room.Status,
		GameStatus: int32(room.GameStatus),
		Frame:      room.Frame,
		Snakes:     snakes,
		Food:       food,
	}, nil
}

// PlayerOperation 玩家游戏操作（方向变更）。
func (s *GameServerImpl) PlayerOperation(ctx context.Context, req *pb.PlayerOperationRequest) (*pb.PlayerOperationResponse, error) {
	playerID := req.GetPlayerId()
	roomID := req.GetRoomId()
	direction := int(req.GetDirection())

	logger.Info("gRPC PlayerOperation", "player_id", playerID, "room_id", roomID, "direction", direction)

	// 参数校验
	if playerID == 0 {
		logger.Warn("gRPC PlayerOperation 参数无效", "player_id", playerID)
		return &pb.PlayerOperationResponse{Code: CodeInvalidParam, Msg: "参数无效"}, nil
	}

	// 方向合法性校验
	if direction < DirUp || direction > DirRight {
		logger.Warn("gRPC PlayerOperation 方向非法", "player_id", playerID, "direction", direction)
		return &pb.PlayerOperationResponse{Code: CodeInvalidParam, Msg: "方向参数无效"}, nil
	}

	// 如果 roomID 为空，通过玩家 ID 查找房间
	if roomID == "" {
		var ok bool
		roomID, ok = s.roomManager.GetPlayerRoom(playerID)
		if !ok {
			logger.Warn("gRPC PlayerOperation 玩家不在任何房间", "player_id", playerID)
			return &pb.PlayerOperationResponse{Code: CodeRoomNotFound, Msg: "玩家未加入房间"}, nil
		}
	}

	// 查找房间
	room, ok := s.roomManager.GetRoom(roomID)
	if !ok {
		logger.Warn("gRPC PlayerOperation 房间不存在", "room_id", roomID)
		return &pb.PlayerOperationResponse{Code: CodeRoomNotFound, Msg: "房间不存在"}, nil
	}

	// 调用房间的 HandlePlayerOperation 处理方向操作
	room.HandlePlayerOperation(playerID, direction)

	logger.Info("gRPC PlayerOperation 成功", "player_id", playerID, "room_id", roomID, "direction", direction)
	return &pb.PlayerOperationResponse{Code: CodeSuccess, Msg: "操作成功"}, nil
}
