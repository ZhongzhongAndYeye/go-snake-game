// 游戏主循环 — 房间级游戏生命周期管理、帧循环、碰撞检测

package game

import (
	"errors"
	"sort"
	"time"

	"go-snake-game/internal/dao"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"google.golang.org/protobuf/proto"
)

const (
	frameInterval = 100 * time.Millisecond // 游戏帧间隔，10 FPS
)

// StartGame 初始化并开始游戏。
// 1. 校验房间状态（玩家数、是否已在游戏中）
// 2. 为每个玩家创建 Snake 实例，分配不同出生位置
// 3. 生成第一个食物
// 4. 设置游戏状态为进行中
// 5. 启动后台 goroutine 运行游戏主循环
func (r *Room) StartGame() error {
	r.mu.Lock()

	// 校验房间状态（玩家数、是否已在游戏中）
	if err := r.setRoomPlayingLocked(); err != nil {
		r.mu.Unlock()
		return err
	}

	// 为每个玩家创建蛇，分配不同出生位置
	for _, p := range r.Players {
		r.Snakes[p.PlayerID] = NewSnake(p.PlayerID, p.Nickname)
	}

	// 生成第一个食物
	food := GenerateFood(r.snakeList(), r.MapWidth, r.MapHeight)
	if food == nil {
		r.mu.Unlock()
		logger.Warn("游戏初始化失败：无法生成食物", "room_id", r.RoomID)
		return errors.New("无法生成食物")
	}
	r.CurrentFood = food

	// 设置游戏状态为进行中，启动帧定时器
	r.GameStatus = GameStatusPlaying
	r.ticker = time.NewTicker(frameInterval)
	r.stopCh = make(chan struct{})

	r.mu.Unlock()

	// 广播游戏开始消息（MsgID=3002）
	r.broadcastGameStart()

	logger.Info("游戏开始", "room_id", r.RoomID, "player_count", len(r.Players))

	// 启动后台游戏主循环
	go r.gameLoop()

	return nil
}

// EndGame 结束游戏。
// 1. 停止帧定时器，关闭停止通道
// 2. 设置游戏状态和房间状态为已结束
// 3. 计算最终排名，更新玩家历史最高分到数据库
func (r *Room) EndGame() {
	r.mu.Lock()

	if r.Status != RoomStatusPlaying {
		r.mu.Unlock()
		return
	}

	// 停止定时器
	if r.ticker != nil {
		r.ticker.Stop()
		r.ticker = nil
	}

	// 关闭停止通道，通知主循环退出
	select {
	case <-r.stopCh:
		// 通道已关闭，无需重复关闭
	default:
		close(r.stopCh)
	}

	r.GameStatus = GameStatusEnded
	r.EndTime = time.Now()     // 记录结束时间，供定时清理判断
	_ = r.setRoomEndedLocked() // 设置房间状态为已结束

	// 计算玩家最终得分并更新数据库
	for _, p := range r.Players {
		if snake, ok := r.Snakes[p.PlayerID]; ok {
			p.Score = int32(snake.Score)
			// 异步更新数据库最高分
			go func(pid uint64, score int) {
				if err := dao.UpdatePlayerScore(pid, score); err != nil {
					logger.Warn("更新玩家最高分失败", "player_id", pid, "score", score, "error", err)
				}
			}(p.PlayerID, snake.Score)
		}
	}

	r.mu.Unlock()

	// 广播游戏结束消息（MsgID=3004）
	r.broadcastGameOver()

	logger.Info("游戏结束", "room_id", r.RoomID)
}

// HandlePlayerOperation 线程安全地处理玩家操作（改变蛇的方向）。
// 游戏未开始或已结束则忽略操作。
func (r *Room) HandlePlayerOperation(playerID uint64, direction int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.GameStatus != GameStatusPlaying {
		return
	}

	snake, ok := r.Snakes[playerID]
	if !ok || !snake.IsAlive {
		return
	}

	snake.ChangeDirection(direction)
}

// gameLoop 后台游戏主循环。
// 以固定帧率（10 FPS）执行：蛇移动 → 碰撞检测 → 吃食物 → 刷新食物 → 游戏结束判定。
func (r *Room) gameLoop() {
	r.mu.Lock()
	ticker := r.ticker
	stopCh := r.stopCh
	r.mu.Unlock()

	for {
		select {
		case <-ticker.C:
			r.tick()
		case <-stopCh:
			return
		}
	}
}

// tick 执行一帧游戏逻辑。
func (r *Room) tick() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.GameStatus != GameStatusPlaying {
		return
	}

	r.Frame++
	aliveCount := 0
	for _, snake := range r.Snakes {
		if !snake.IsAlive {
			continue
		}
		aliveCount++

		// 1. 蛇移动（未吃到食物，不增长）
		snake.Move(false)

		// 2. 碰撞检测
		if CheckWallCollision(snake, r.MapWidth, r.MapHeight) {
			snake.Die()
			logger.Info("玩家撞墙死亡", "player_id", snake.PlayerID, "room_id", r.RoomID)
			continue
		}
		if CheckSelfCollision(snake) {
			snake.Die()
			logger.Info("玩家撞自身死亡", "player_id", snake.PlayerID, "room_id", r.RoomID)
			continue
		}

		// 3. 吃食物检测
		if r.CurrentFood != nil && CheckEatFood(snake, r.CurrentFood) {
			// 吃到食物：蛇身增长，加分数，重新生成食物
			snake.Grow()
			snake.AddScore(r.CurrentFood.ScoreValue)
			logger.Info("玩家吃到食物", "player_id", snake.PlayerID, "score", snake.Score, "room_id", r.RoomID)

			// 生成新食物
			newFood := GenerateFood(r.snakeList(), r.MapWidth, r.MapHeight)
			if newFood != nil {
				r.CurrentFood = newFood
			}
		}
	}

	// 4. 游戏结束判定：所有玩家死亡或只剩一个存活
	if aliveCount <= 1 {
		logger.Info("游戏结束判定：仅剩一个存活玩家或无存活玩家", "room_id", r.RoomID, "alive_count", aliveCount)
		// 结束游戏（在 goroutine 中异步执行，避免死锁）
		go r.EndGame()
	}

	// 5. 广播帧状态同步（MsgID=3003）
	// 释放锁后再进行 gRPC 调用，避免长时间持有锁影响其他操作
	r.broadcastGameStateSync()

	// 记录帧日志（每 10 帧输出一次，便于观察主循环运行状态）
	if r.Frame%10 == 0 {
		aliveNames := make([]uint64, 0)
		for _, snake := range r.Snakes {
			if snake.IsAlive {
				aliveNames = append(aliveNames, snake.PlayerID)
			}
		}
		logger.Info("游戏帧",
			"room_id", r.RoomID,
			"frame", r.Frame,
			"alive_players", aliveNames,
			"food_pos", r.CurrentFood,
		)
	}
}

// snakeList 返回当前房间内所有蛇的切片，供 GenerateFood 使用。
func (r *Room) snakeList() []*Snake {
	snakes := make([]*Snake, 0, len(r.Snakes))
	for _, snake := range r.Snakes {
		snakes = append(snakes, snake)
	}
	return snakes
}

// ---- 下行推送辅助方法 ----

// broadcastGameStart 广播游戏开始消息（MsgID=3002）。
// 获取锁读取数据后立即释放，再执行 gRPC 调用，避免长时间持有锁。
func (r *Room) broadcastGameStart() {
	r.mu.Lock()
	notify := &msg.GameStartNotify{
		MapWidth:  int32(r.MapWidth),
		MapHeight: int32(r.MapHeight),
		Snakes:    buildSnakeStates(r.Snakes),
		Food:      buildFoodState(r.CurrentFood),
	}
	r.mu.Unlock()

	body, err := proto.Marshal(notify)
	if err != nil {
		logger.Error("序列化 GameStartNotify 失败", "room_id", r.RoomID, "error", err)
		return
	}

	GlobalGatewayClient.BroadcastRoomMsg(r.RoomID, network.MsgIDGameStartNotify, body)
}

// broadcastGameStateSync 广播帧状态同步消息（MsgID=3003）。
// 调用前 r.mu 已持有，构建数据后释放锁再进行 gRPC 调用，避免阻塞其他操作。
func (r *Room) broadcastGameStateSync() {
	sync := &msg.GameStateSync{
		Frame:  r.Frame,
		Snakes: buildSnakeStates(r.Snakes),
		Food:   buildFoodState(r.CurrentFood),
	}

	body, err := proto.Marshal(sync)
	if err != nil {
		logger.Error("序列化 GameStateSync 失败", "room_id", r.RoomID, "frame", r.Frame, "error", err)
		return
	}

	// 释放锁后再进行 gRPC 调用
	r.mu.Unlock()
	GlobalGatewayClient.BroadcastRoomMsg(r.RoomID, network.MsgIDGameStateSync, body)
	r.mu.Lock()
}

// broadcastGameOver 广播游戏结束消息（MsgID=3004）。
// 获取锁读取数据后立即释放，再执行 gRPC 调用，避免长时间持有锁。
func (r *Room) broadcastGameOver() {
	r.mu.Lock()
	notify := &msg.GameOverNotify{
		Ranks: buildPlayerRanks(r.Players, r.Snakes),
	}
	r.mu.Unlock()

	body, err := proto.Marshal(notify)
	if err != nil {
		logger.Error("序列化 GameOverNotify 失败", "room_id", r.RoomID, "error", err)
		return
	}

	GlobalGatewayClient.BroadcastRoomMsg(r.RoomID, network.MsgIDGameOverNotify, body)
}

// buildSnakeStates 将 Snakes map 转换为 proto 的 SnakeState 列表。
// 调用方必须持有 r.mu 锁。
func buildSnakeStates(snakes map[uint64]*Snake) []*msg.SnakeState {
	states := make([]*msg.SnakeState, 0, len(snakes))
	for _, snake := range snakes {
		body := make([]*msg.Point, 0, len(snake.Body))
		for _, pt := range snake.Body {
			body = append(body, &msg.Point{X: int32(pt.X), Y: int32(pt.Y)})
		}
		states = append(states, &msg.SnakeState{
			PlayerId: snake.PlayerID,
			Nickname: snake.Nickname,
			Body:     body,
			Score:    int32(snake.Score),
			IsAlive:  snake.IsAlive,
		})
	}
	return states
}

// buildFoodState 将 Food 转换为 proto 的 FoodState。
// 调用方必须持有 r.mu 锁。
func buildFoodState(food *Food) *msg.FoodState {
	if food == nil {
		return nil
	}
	return &msg.FoodState{
		Position: &msg.Point{X: int32(food.Position.X), Y: int32(food.Position.Y)},
		Score:    int32(food.ScoreValue),
	}
}

// buildPlayerRanks 根据玩家分数构建排名列表（按分数降序排列）。
// 调用方必须持有 r.mu 锁。
func buildPlayerRanks(players []*PlayerInfo, snakes map[uint64]*Snake) []*msg.PlayerRank {
	type playerScore struct {
		playerID uint64
		nickname string
		score    int
	}

	scores := make([]playerScore, 0, len(players))
	for _, p := range players {
		score := 0
		if snake, ok := snakes[p.PlayerID]; ok {
			score = snake.Score
		}
		scores = append(scores, playerScore{
			playerID: p.PlayerID,
			nickname: p.Nickname,
			score:    score,
		})
	}

	// 按分数降序排列
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	ranks := make([]*msg.PlayerRank, 0, len(scores))
	for i, ps := range scores {
		ranks = append(ranks, &msg.PlayerRank{
			PlayerId: ps.playerID,
			Nickname: ps.nickname,
			Score:    int32(ps.score),
			Rank:     int32(i + 1),
		})
	}
	return ranks
}
