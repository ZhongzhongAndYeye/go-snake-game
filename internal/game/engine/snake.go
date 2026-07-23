// Package engine 提供贪吃蛇游戏核心引擎，包括蛇的移动、碰撞检测、食物生成等基础逻辑。
package engine

import (
	"math/rand"
	"time"
)

// 方向枚举，用于标识蛇的当前移动方向或方向变更请求
const (
	DirUp    = 1 // 上：Y 坐标减 1
	DirDown  = 2 // 下：Y 坐标加 1
	DirLeft  = 3 // 左：X 坐标减 1
	DirRight = 4 // 右：X 坐标加 1
)

// 游戏状态枚举，用于标识房间内游戏的当前阶段
const (
	GameStatusNotStarted = 1 // 未开始：玩家已加入房间，游戏尚未启动
	GameStatusPlaying    = 2 // 进行中：游戏正在运行，蛇按固定周期移动
	GameStatusPaused     = 3 // 暂停：游戏暂停，蛇停止移动
	GameStatusEnded      = 4 // 已结束：游戏结束（分出胜负或平局）
)

// 地图常量，定义游戏网格的默认边界范围
const (
	DefaultMapWidth  = 20 // 地图宽度（X 轴范围：0 ~ 19）
	DefaultMapHeight = 20 // 地图高度（Y 轴范围：0 ~ 19）
)

// Point 表示游戏网格上的一个坐标点
type Point struct {
	X int // 横坐标（列），范围 0 ~ MapWidth-1
	Y int // 纵坐标（行），范围 0 ~ MapHeight-1
}

// Snake 表示一条贪吃蛇，由单个玩家控制
type Snake struct {
	PlayerID   uint64    // 玩家 ID，唯一标识所属玩家
	Nickname   string    // 玩家昵称，用于显示
	Body       []Point   // 蛇身坐标列表，索引 0 为蛇头，后续为身体和尾部
	Direction  int       // 当前移动方向，取值为 DirUp / DirDown / DirLeft / DirRight
	Score      int       // 当前得分，每吃到一个食物增加
	IsAlive    bool      // 是否存活，蛇撞墙或撞自身后置为 false
	LastOpTime time.Time // 上次操作时间，用于频率限制
}

// 操作频率限制，每 100ms 最多接受 1 次方向操作。
const OpRateLimitInterval = 100 * time.Millisecond

// Food 表示地图上的一个食物
type Food struct {
	Position   Point // 食物所在坐标
	ScoreValue int   // 吃掉该食物获得的分数，默认 10 分
}

// spawnPositions 是预定义的玩家出生位置列表。
// 每个元素包含蛇头坐标和初始朝向，玩家之间相隔足够距离避免开局重叠。
// 按 playerID % len(spawnPositions) 分配，确保不同玩家出生在不同位置。
var spawnPositions = []struct {
	head Point // 蛇头出生坐标
	dir  int   // 初始朝向
}{
	{head: Point{X: 5, Y: 5}, dir: DirRight},  // 玩家 A：出生在左上角 (5,5)，朝右，蛇身向左延伸至 (3,5)
	{head: Point{X: 14, Y: 14}, dir: DirLeft}, // 玩家 B：出生在右下角 (14,14)，朝左，蛇身向右延伸至 (16,14)
}

// NewSnake 创建一条新蛇，根据玩家 ID 分配不同的出生位置。
// 蛇身初始长度为 3 节，出生时存活。
func NewSnake(playerID uint64, nickname string) *Snake {
	// 根据玩家 ID 取模，从预定义的出生位置列表中选择一个
	idx := int(playerID) % len(spawnPositions)
	sp := spawnPositions[idx]

	head := sp.head
	// 根据初始朝向构建 3 节蛇身：蛇头在最前，身体和尾部依次向后延伸
	var body []Point
	switch sp.dir {
	case DirRight:
		body = []Point{head, {X: head.X - 1, Y: head.Y}, {X: head.X - 2, Y: head.Y}}
	case DirLeft:
		body = []Point{head, {X: head.X + 1, Y: head.Y}, {X: head.X + 2, Y: head.Y}}
	case DirUp:
		body = []Point{head, {X: head.X, Y: head.Y + 1}, {X: head.X, Y: head.Y + 2}}
	case DirDown:
		body = []Point{head, {X: head.X, Y: head.Y - 1}, {X: head.X, Y: head.Y - 2}}
	}

	return &Snake{
		PlayerID:  playerID,
		Nickname:  nickname,
		Body:      body,
		Direction: sp.dir,
		Score:     0,
		IsAlive:   true,
	}
}

// Move 控制蛇沿当前方向前进一步。
// 计算新的蛇头坐标，插入到 Body 最前面。
// grow 参数控制是否移除尾部：
//   - true：保留尾部（蛇身增长），吃到食物时传入 true
//   - false：移除尾部（保持长度不变），普通移动时传入 false
//
// 返回新的蛇头坐标，供调用方判断是否吃到食物或发生碰撞。
func (s *Snake) Move(grow bool) Point {
	head := s.Body[0]
	var newHead Point
	switch s.Direction {
	case DirUp:
		newHead = Point{X: head.X, Y: head.Y - 1}
	case DirDown:
		newHead = Point{X: head.X, Y: head.Y + 1}
	case DirLeft:
		newHead = Point{X: head.X - 1, Y: head.Y}
	case DirRight:
		newHead = Point{X: head.X + 1, Y: head.Y}
	}

	if grow {
		// 吃到食物：头部插入，尾部保留，蛇身长度 +1
		s.Body = append([]Point{newHead}, s.Body...)
	} else {
		// 未吃到食物：头部插入，尾部弹出，蛇身长度不变
		s.Body = append([]Point{newHead}, s.Body[:len(s.Body)-1]...)
	}
	return newHead
}

// Grow 让蛇身增长一节，在蛇吃到食物后调用。
// 实现方式：复制当前尾部坐标追加到末尾，蛇身长度 +1。
func (s *Snake) Grow() {
	tail := s.Body[len(s.Body)-1]
	s.Body = append(s.Body, tail)
}

// ChangeDirection 尝试改变蛇的移动方向。
// 禁止直接反向（例如正在向右时不能直接向左），防止蛇头穿体自杀。
// 返回 true 表示方向变更成功，false 表示反向变更被拒绝。
func (s *Snake) ChangeDirection(newDir int) bool {
	opposite := map[int]int{
		DirUp:    DirDown,
		DirDown:  DirUp,
		DirLeft:  DirRight,
		DirRight: DirLeft,
	}
	if s.Direction == opposite[newDir] {
		return false
	}
	s.Direction = newDir
	return true
}

// CheckWallCollision 检测蛇头是否超出地图边界。
// 超出范围（X < 0 或 X >= mapWidth 或 Y < 0 或 Y >= mapHeight）返回 true。
func CheckWallCollision(snake *Snake, mapWidth, mapHeight int) bool {
	head := snake.Body[0]
	return head.X < 0 || head.X >= mapWidth || head.Y < 0 || head.Y >= mapHeight
}

// CheckSelfCollision 检测蛇头是否撞到自己的身体。
// 从索引 1 开始遍历（排除蛇头自身），判断是否有坐标与蛇头重叠。
// 返回 true 表示发生自身碰撞，蛇应死亡。
func CheckSelfCollision(snake *Snake) bool {
	head := snake.Body[0]
	for i := 1; i < len(snake.Body); i++ {
		if head.X == snake.Body[i].X && head.Y == snake.Body[i].Y {
			return true
		}
	}
	return false
}

// CheckInterSnakeCollision 检测蛇头是否撞到对方蛇的身体。
// 房间内只有两条蛇，直接比较蛇头与对方蛇身每一节是否重叠即可。
// 返回 true 表示撞到对方，该蛇应死亡。
func CheckInterSnakeCollision(snake *Snake, opponent *Snake) bool {
	head := snake.Body[0]
	for _, body := range opponent.Body {
		if head.X == body.X && head.Y == body.Y {
			return true
		}
	}
	return false
}

// CheckEatFood 检测蛇头是否吃到了食物。
// 判断蛇头坐标与食物坐标是否完全重合，重合返回 true。
func CheckEatFood(snake *Snake, food *Food) bool {
	head := snake.Body[0]
	return head.X == food.Position.X && head.Y == food.Position.Y
}

// GenerateFood 在地图范围内随机生成一个食物。
// 确保食物坐标不与任何蛇的身体重叠，避免生成在蛇身上。
// 最多重试 100 次，若仍无法找到空闲位置则返回 nil。
func GenerateFood(snakes []*Snake, mapWidth, mapHeight int) *Food {
	for attempt := 0; attempt < 100; attempt++ {
		pos := Point{
			X: rand.Intn(mapWidth),
			Y: rand.Intn(mapHeight),
		}
		// 检查是否与任何蛇的身体重叠
		overlap := false
		for _, snake := range snakes {
			for _, body := range snake.Body {
				if pos.X == body.X && pos.Y == body.Y {
					overlap = true
					break
				}
			}
			if overlap {
				break
			}
		}
		if !overlap {
			return &Food{Position: pos, ScoreValue: 10}
		}
	}
	// 尝试 100 次均未找到空闲位置，返回 nil
	return nil
}

// Die 将蛇标记为死亡状态。碰撞检测方确认碰撞后调用此方法。
func (s *Snake) Die() {
	s.IsAlive = false
}

// AddScore 为蛇增加指定分数，在蛇吃到食物后调用。
func (s *Snake) AddScore(points int) {
	s.Score += points
}
