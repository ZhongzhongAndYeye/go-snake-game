// 游戏服 gRPC 服务端实现
// 包含匹配、取消匹配、房间信息查询等 RPC 接口

package game

import (
	"context"
	"errors"

	"go-snake-game/internal/game/engine"
	"go-snake-game/internal/game/match"
	"go-snake-game/internal/game/room"
	"go-snake-game/internal/game/rpc"
	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	pb "go-snake-game/pkg/proto/rpc"
	"go-snake-game/pkg/utils"
)

// GameServerImpl 游戏 gRPC 服务端实现。
// 嵌入 UnimplementedGameServiceServer 保证向前兼容，
// 持有匹配管理器和房间管理器实例完成实际业务逻辑。
type GameServerImpl struct {
	pb.UnimplementedGameServiceServer
	matchManager *match.MatchManager
	roomManager  *room.RoomManager
}

// NewGameServer 创建游戏 gRPC 服务端实例。
// 注入匹配管理器与房间管理器全局单例。
func NewGameServer() *GameServerImpl {
	return &GameServerImpl{
		matchManager: match.GetMatchManager(),
		roomManager:  room.GetRoomManager(),
	}
}

// InitGatewayRpcClient 初始化网关 gRPC 客户端（代理到 rpc 子包）。
// 保留此函数避免 cmd/game 直接依赖 rpc 子包。
func InitGatewayRpcClient(addr string) {
	rpc.InitGatewayRpcClient(addr)
}

// StartMatch 发起匹配。
func (s *GameServerImpl) StartMatch(ctx context.Context, req *pb.StartMatchRequest) (*pb.StartMatchResponse, error) {
	playerID := req.GetPlayerId()
	nickname := req.GetNickname()
	traceLog := logger.WithTraceID(utils.GetTraceIDFromMetadata(ctx))

	traceLog.Info("gRPC StartMatch", "player_id", playerID, "nickname", nickname)

	// 参数校验
	if playerID == 0 {
		traceLog.Warn("gRPC StartMatch 参数无效", "player_id", playerID)
		return &pb.StartMatchResponse{Code: errcode.ErrParam, Msg: "玩家 ID 不能为空"}, nil
	}

	// 调用匹配管理器加入队列
	roomID, isMatched, waitingPlayerID, waitingNickname, err := s.matchManager.AddToMatchQueue(playerID, nickname)
	if err != nil {
		traceLog.Warn("gRPC StartMatch 匹配失败", "player_id", playerID, "error", err.Error())
		return &pb.StartMatchResponse{Code: errcode.ErrMatchFailed, Msg: "匹配失败，请稍后重试"}, nil
	}

	if isMatched {
		// 匹配成功，创建房间并将两名玩家加入房间
		rm := s.roomManager.CreateRoom(roomID)
		_ = rm.AddPlayer(waitingPlayerID, waitingNickname)
		_ = rm.AddPlayer(playerID, nickname)

		// 绑定玩家到房间
		s.roomManager.BindPlayerToRoom(waitingPlayerID, roomID)
		s.roomManager.BindPlayerToRoom(playerID, roomID)

		// 房间人满（2 人），自动开始游戏
		if err := rm.StartGame(); err != nil {
			traceLog.Warn("gRPC StartMatch 开始游戏失败", "room_id", roomID, "error", err.Error())
		}

		traceLog.Info("gRPC StartMatch 匹配成功", "player_id", playerID, "room_id", roomID)
		return &pb.StartMatchResponse{
			Code:      errcode.OK,
			Msg:       "匹配成功",
			RoomId:    roomID,
			IsMatched: true,
		}, nil
	}

	// 匹配等待中
	traceLog.Info("gRPC StartMatch 进入等待", "player_id", playerID)
	return &pb.StartMatchResponse{
		Code:      errcode.OK,
		Msg:       "已进入匹配队列",
		RoomId:    "",
		IsMatched: false,
	}, nil
}

// CancelMatch 取消匹配。
func (s *GameServerImpl) CancelMatch(ctx context.Context, req *pb.CancelMatchRequest) (*pb.CancelMatchResponse, error) {
	playerID := req.GetPlayerId()
	traceLog := logger.WithTraceID(utils.GetTraceIDFromMetadata(ctx))

	traceLog.Info("gRPC CancelMatch", "player_id", playerID)

	// 参数校验
	if playerID == 0 {
		traceLog.Warn("gRPC CancelMatch 参数无效", "player_id", playerID)
		return &pb.CancelMatchResponse{Code: errcode.ErrParam, Msg: "玩家 ID 不能为空"}, nil
	}

	// 调用匹配管理器移除玩家
	err := s.matchManager.RemoveFromMatchQueue(playerID)
	if err != nil {
		if errors.Is(err, match.ErrPlayerNotInQueue) {
			traceLog.Warn("gRPC CancelMatch 玩家不在队列", "player_id", playerID)
			return &pb.CancelMatchResponse{Code: errcode.ErrMatchFailed, Msg: "您不在匹配队列中"}, nil
		}
		traceLog.Warn("gRPC CancelMatch 取消失败", "player_id", playerID, "error", err.Error())
		return &pb.CancelMatchResponse{Code: errcode.ErrMatchFailed, Msg: "取消匹配失败，请稍后重试"}, nil
	}

	traceLog.Info("gRPC CancelMatch 成功", "player_id", playerID)
	return &pb.CancelMatchResponse{Code: errcode.OK, Msg: "取消匹配成功"}, nil
}

// GetRoomInfo 获取当前房间信息。
func (s *GameServerImpl) GetRoomInfo(ctx context.Context, req *pb.GetRoomInfoRequest) (*pb.GetRoomInfoResponse, error) {
	roomID := req.GetRoomId()
	traceLog := logger.WithTraceID(utils.GetTraceIDFromMetadata(ctx))

	traceLog.Info("gRPC GetRoomInfo", "room_id", roomID)

	// 参数校验
	if roomID == "" {
		traceLog.Warn("gRPC GetRoomInfo 参数无效", "room_id", roomID)
		return &pb.GetRoomInfoResponse{Code: errcode.ErrParam, Msg: "房间 ID 不能为空"}, nil
	}

	// 查询房间管理器
	rm, ok := s.roomManager.GetRoom(roomID)
	if !ok {
		traceLog.Warn("gRPC GetRoomInfo 房间不存在", "room_id", roomID)
		return &pb.GetRoomInfoResponse{Code: errcode.ErrRoomNotExist, Msg: "房间不存在"}, nil
	}

	rm.Lock()
	defer rm.Unlock()

	// 转换玩家列表
	players := make([]*pb.PlayerInfo, 0, len(rm.Players))
	for _, p := range rm.Players {
		players = append(players, &pb.PlayerInfo{
			PlayerId: p.PlayerID,
			Nickname: p.Nickname,
			Score:    p.Score,
		})
	}

	// 转换蛇状态
	snakes := make([]*pb.SnakeState, 0, len(rm.Snakes))
	for _, snake := range rm.Snakes {
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
	if rm.CurrentFood != nil {
		rf := rm.CurrentFood
		food = &pb.FoodState{
			Position: &pb.Point{X: int32(rf.Position.X), Y: int32(rf.Position.Y)},
			Score:    int32(rf.ScoreValue),
		}
	}

	traceLog.Info("gRPC GetRoomInfo 成功", "room_id", roomID, "player_count", len(players), "game_status", rm.GameStatus)
	return &pb.GetRoomInfoResponse{
		Code:       errcode.OK,
		Msg:        "获取房间信息成功",
		RoomId:     roomID,
		Players:    players,
		Status:     rm.Status,
		GameStatus: int32(rm.GameStatus),
		Frame:      rm.Frame,
		Snakes:     snakes,
		Food:       food,
	}, nil
}

// PlayerOperation 玩家游戏操作（方向变更）。
func (s *GameServerImpl) PlayerOperation(ctx context.Context, req *pb.PlayerOperationRequest) (*pb.PlayerOperationResponse, error) {
	playerID := req.GetPlayerId()
	roomID := req.GetRoomId()
	direction := int(req.GetDirection())
	traceLog := logger.WithTraceID(utils.GetTraceIDFromMetadata(ctx))

	traceLog.Info("gRPC PlayerOperation", "player_id", playerID, "room_id", roomID, "direction", direction)

	// 参数校验
	if playerID == 0 {
		traceLog.Warn("gRPC PlayerOperation 参数无效", "player_id", playerID)
		return &pb.PlayerOperationResponse{Code: errcode.ErrParam, Msg: "参数无效"}, nil
	}

	// 方向合法性校验
	if direction < engine.DirUp || direction > engine.DirRight {
		traceLog.Warn("gRPC PlayerOperation 方向非法", "player_id", playerID, "direction", direction)
		return &pb.PlayerOperationResponse{Code: errcode.ErrParam, Msg: "方向参数无效"}, nil
	}

	// 如果 roomID 为空，通过玩家 ID 查找房间
	if roomID == "" {
		var ok bool
		roomID, ok = s.roomManager.GetPlayerRoom(playerID)
		if !ok {
			traceLog.Warn("gRPC PlayerOperation 玩家不在任何房间", "player_id", playerID)
			return &pb.PlayerOperationResponse{Code: errcode.ErrRoomNotExist, Msg: "玩家未加入房间"}, nil
		}
	}

	// 查找房间
	rm, ok := s.roomManager.GetRoom(roomID)
	if !ok {
		traceLog.Warn("gRPC PlayerOperation 房间不存在", "room_id", roomID)
		return &pb.PlayerOperationResponse{Code: errcode.ErrRoomNotExist, Msg: "房间不存在"}, nil
	}

	// 调用房间的 HandlePlayerOperation 处理方向操作
	rm.HandlePlayerOperation(playerID, direction)

	traceLog.Info("gRPC PlayerOperation 成功", "player_id", playerID, "room_id", roomID, "direction", direction)
	return &pb.PlayerOperationResponse{Code: errcode.OK, Msg: "操作成功"}, nil
}

// PlayerOffline 玩家离线通知。
// 网关在玩家 WebSocket 断开连接时调用此接口。
// 游戏服根据玩家当前状态处理：
//   - 游戏中：标记该玩家蛇死亡，触发游戏结束判定
//   - 匹配中：从匹配队列移除
func (s *GameServerImpl) PlayerOffline(ctx context.Context, req *pb.PlayerOfflineRequest) (*pb.PlayerOfflineResponse, error) {
	playerID := req.GetPlayerId()
	roomID := req.GetRoomId()
	traceLog := logger.WithTraceID(utils.GetTraceIDFromMetadata(ctx))

	traceLog.Info("gRPC PlayerOffline", "player_id", playerID, "room_id", roomID)

	// 参数校验
	if playerID == 0 {
		traceLog.Warn("gRPC PlayerOffline 参数无效", "player_id", playerID)
		return &pb.PlayerOfflineResponse{Code: errcode.ErrParam, Msg: "玩家 ID 不能为空"}, nil
	}

	// 如果 roomID 为空，通过玩家 ID 查找房间
	if roomID == "" {
		var ok bool
		roomID, ok = s.roomManager.GetPlayerRoom(playerID)
		if !ok {
			// 玩家不在任何房间，可能正在匹配队列中，尝试从匹配队列移除
			traceLog.Info("gRPC PlayerOffline 玩家不在房间，尝试从匹配队列移除", "player_id", playerID)
			if err := s.matchManager.RemoveFromMatchQueue(playerID); err != nil {
				if errors.Is(err, match.ErrPlayerNotInQueue) {
					traceLog.Info("gRPC PlayerOffline 玩家既不在房间也不在匹配队列", "player_id", playerID)
					return &pb.PlayerOfflineResponse{Code: errcode.OK, Msg: "玩家不在任何房间或队列中"}, nil
				}
				traceLog.Warn("gRPC PlayerOffline 从匹配队列移除失败", "player_id", playerID, "error", err)
				return &pb.PlayerOfflineResponse{Code: errcode.ErrSystem, Msg: "从匹配队列移除失败"}, nil
			}
			traceLog.Info("gRPC PlayerOffline 已从匹配队列移除", "player_id", playerID)
			return &pb.PlayerOfflineResponse{Code: errcode.OK, Msg: "已从匹配队列移除"}, nil
		}
	}

	// 查找房间
	rm, ok := s.roomManager.GetRoom(roomID)
	if !ok {
		traceLog.Warn("gRPC PlayerOffline 房间不存在", "room_id", roomID)
		return &pb.PlayerOfflineResponse{Code: errcode.ErrRoomNotExist, Msg: "房间不存在"}, nil
	}

	// 处理房间内的离线逻辑
	rm.Lock()

	// 标记玩家离线
	for _, p := range rm.Players {
		if p.PlayerID == playerID {
			p.IsOnline = false
			break
		}
	}

	// 如果游戏正在进行，标记蛇死亡并触发结束判定
	if rm.GameStatus == engine.GameStatusPlaying {
		if snake, ok := rm.Snakes[playerID]; ok && snake.IsAlive {
			snake.Die()
			traceLog.Info("玩家离线，标记蛇死亡", "player_id", playerID, "room_id", roomID)

			// 检查是否所有存活蛇都已死亡（只剩离线玩家死亡或全部死亡）
			aliveCount := 0
			for _, s := range rm.Snakes {
				if s.IsAlive {
					aliveCount++
				}
			}
			if aliveCount <= 1 {
				traceLog.Info("玩家离线触发游戏结束判定", "room_id", roomID, "alive_count", aliveCount)
				rm.Unlock()
				// 异步结束游戏，避免死锁
				rm.EndGame()
				traceLog.Info("gRPC PlayerOffline 成功，已触发游戏结束", "player_id", playerID, "room_id", roomID)
				return &pb.PlayerOfflineResponse{Code: errcode.OK, Msg: "玩家离线，游戏已结束"}, nil
			}
		}
	}

	rm.Unlock()

	traceLog.Info("gRPC PlayerOffline 成功", "player_id", playerID, "room_id", roomID)
	return &pb.PlayerOfflineResponse{Code: errcode.OK, Msg: "玩家离线处理成功"}, nil
}

// GetGlobalRank 查询全服排行榜 Top100。
// 调用排行榜工具查询 Redis ZSet，封装返回。
func (s *GameServerImpl) GetGlobalRank(ctx context.Context, req *pb.GetGlobalRankRequest) (*pb.GetGlobalRankResponse, error) {
	traceLog := logger.WithTraceID(utils.GetTraceIDFromMetadata(ctx))
	traceLog.Info("gRPC GetGlobalRank")

	// 查询 Top100
	items, err := utils.GetTopN(100)
	if err != nil {
		traceLog.Error("gRPC GetGlobalRank 查询排行榜失败", "error", err)
		return &pb.GetGlobalRankResponse{Code: errcode.ErrSystem, Msg: "查询排行榜失败"}, nil
	}

	// 转换为 proto 格式
	list := make([]*pb.RankItem, 0, len(items))
	for _, item := range items {
		list = append(list, &pb.RankItem{
			PlayerId: item.PlayerID,
			Score:    int32(item.Score),
			Rank:     int32(item.Rank),
		})
	}

	traceLog.Info("gRPC GetGlobalRank 成功", "count", len(list))
	return &pb.GetGlobalRankResponse{
		Code: errcode.OK,
		Msg:  "查询成功",
		List: list,
	}, nil
}
