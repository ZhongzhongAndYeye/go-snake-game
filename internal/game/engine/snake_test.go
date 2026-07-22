package engine

import (
	"testing"
)

// ---------- 正常移动 ----------

func TestSnake_Move_Right(t *testing.T) {
	s := &Snake{
		Body:      []Point{{X: 5, Y: 5}, {X: 4, Y: 5}, {X: 3, Y: 5}},
		Direction: DirRight,
	}
	oldTail := s.Body[len(s.Body)-1]

	s.Move(false)

	// 新蛇头应在旧蛇头右边一格
	if s.Body[0].X != 6 || s.Body[0].Y != 5 {
		t.Errorf("头部应右移一格到 (6,5)，got (%d,%d)", s.Body[0].X, s.Body[0].Y)
	}
	// 尾部应已弹出
	if s.Body[len(s.Body)-1].X == oldTail.X && s.Body[len(s.Body)-1].Y == oldTail.Y {
		t.Errorf("尾部应已弹出，但仍在 Body 末尾")
	}
	// 长度不变
	if len(s.Body) != 3 {
		t.Errorf("未吃食物时长度应保持 3，got %d", len(s.Body))
	}
}

func TestSnake_Move_Grow(t *testing.T) {
	s := NewSnake(1, "test")
	oldLen := len(s.Body)

	s.Move(true)

	// 长度应 +1
	if len(s.Body) != oldLen+1 {
		t.Errorf("吃食物后长度应 +1，got %d, want %d", len(s.Body), oldLen+1)
	}
}

// ---------- 转向 ----------

func TestSnake_ChangeDirection_Valid(t *testing.T) {
	s := NewSnake(1, "test")
	// 初始朝右，改为朝上
	if !s.ChangeDirection(DirUp) {
		t.Errorf("向右→向上应为合法转向")
	}
	if s.Direction != DirUp {
		t.Errorf("Direction 应为 DirUp，got %d", s.Direction)
	}
}

func TestSnake_ChangeDirection_Reverse(t *testing.T) {
	s := &Snake{
		Body:      []Point{{X: 5, Y: 5}, {X: 4, Y: 5}, {X: 3, Y: 5}},
		Direction: DirRight,
	}
	// 初始朝右，改为向左 → 反向，应拒绝
	if s.ChangeDirection(DirLeft) {
		t.Errorf("向右→向左应为反向，应返回 false")
	}
	if s.Direction != DirRight {
		t.Errorf("反向被拒绝后 Direction 应保持 DirRight，got %d", s.Direction)
	}
}

func TestSnake_ChangeDirection_AllReversePairs(t *testing.T) {
	tests := []struct {
		current int
		reverse int
		name    string
	}{
		{DirUp, DirDown, "上→下"},
		{DirDown, DirUp, "下→上"},
		{DirLeft, DirRight, "左→右"},
		{DirRight, DirLeft, "右→左"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Snake{
				Body:      []Point{{X: 5, Y: 5}, {X: 4, Y: 5}, {X: 3, Y: 5}},
				Direction: tt.current,
			}
			if s.ChangeDirection(tt.reverse) {
				t.Errorf("%s 应为反向，应返回 false", tt.name)
			}
		})
	}
}

// ---------- 撞墙检测 ----------

func TestCheckWallCollision_NoCollision(t *testing.T) {
	s := NewSnake(1, "test")
	// 蛇头在 (5,5)，20x20 地图内，不应撞墙
	if CheckWallCollision(s, DefaultMapWidth, DefaultMapHeight) {
		t.Errorf("蛇头在边界内不应撞墙")
	}
}

func TestCheckWallCollision_Top(t *testing.T) {
	s := NewSnake(1, "test")
	s.Body[0] = Point{X: 5, Y: -1} // 超出上边界
	if !CheckWallCollision(s, DefaultMapWidth, DefaultMapHeight) {
		t.Errorf("Y=-1 应视为撞墙")
	}
}

func TestCheckWallCollision_Left(t *testing.T) {
	s := NewSnake(1, "test")
	s.Body[0] = Point{X: -1, Y: 5} // 超出左边界
	if !CheckWallCollision(s, DefaultMapWidth, DefaultMapHeight) {
		t.Errorf("X=-1 应视为撞墙")
	}
}

func TestCheckWallCollision_Bottom(t *testing.T) {
	s := NewSnake(1, "test")
	s.Body[0] = Point{X: 5, Y: 20} // 超出下边界（>= MapHeight）
	if !CheckWallCollision(s, DefaultMapWidth, DefaultMapHeight) {
		t.Errorf("Y=20 应视为撞墙")
	}
}

func TestCheckWallCollision_Right(t *testing.T) {
	s := NewSnake(1, "test")
	s.Body[0] = Point{X: 20, Y: 5} // 超出右边界（>= MapWidth）
	if !CheckWallCollision(s, DefaultMapWidth, DefaultMapHeight) {
		t.Errorf("X=20 应视为撞墙")
	}
}

// ---------- 撞自身检测 ----------

func TestCheckSelfCollision_NoCollision(t *testing.T) {
	s := NewSnake(1, "test")
	// 初始蛇身不重叠
	if CheckSelfCollision(s) {
		t.Errorf("初始蛇身不应自撞")
	}
}

func TestCheckSelfCollision_Collision(t *testing.T) {
	s := NewSnake(1, "test")
	// 构造蛇头与身体重叠的场景
	s.Body = []Point{
		{X: 5, Y: 5}, // 蛇头
		{X: 5, Y: 6}, // 身体
		{X: 5, Y: 5}, // 蛇头撞到自身（与索引 0 以外的坐标重叠）
	}
	if !CheckSelfCollision(s) {
		t.Errorf("蛇头与身体重叠应检测为自撞")
	}
}

// ---------- 蛇间碰撞检测 ----------

func TestCheckInterSnakeCollision_NoCollision(t *testing.T) {
	s1 := NewSnake(1, "p1")
	s2 := NewSnake(2, "p2")
	// 两条蛇出生在不同位置，不应碰撞
	if CheckInterSnakeCollision(s1, s2) {
		t.Errorf("不同出生位置的蛇不应碰撞")
	}
}

func TestCheckInterSnakeCollision_HeadOnBody(t *testing.T) {
	s1 := &Snake{
		PlayerID: 1,
		Body:     []Point{{X: 5, Y: 5}, {X: 4, Y: 5}, {X: 3, Y: 5}},
		IsAlive:  true,
	}
	s2 := &Snake{
		PlayerID: 2,
		Body:     []Point{{X: 10, Y: 10}, {X: 5, Y: 5}, {X: 9, Y: 10}}, // s2 的身体在 (5,5) 与 s1 蛇头重合
		IsAlive:  true,
	}
	if !CheckInterSnakeCollision(s1, s2) {
		t.Errorf("s1 蛇头撞到 s2 身体应返回 true")
	}
}

func TestCheckInterSnakeCollision_HeadToHead(t *testing.T) {
	s1 := &Snake{
		PlayerID: 1,
		Body:     []Point{{X: 5, Y: 5}, {X: 4, Y: 5}, {X: 3, Y: 5}},
		IsAlive:  true,
	}
	s2 := &Snake{
		PlayerID: 2,
		Body:     []Point{{X: 5, Y: 5}, {X: 6, Y: 5}, {X: 7, Y: 5}}, // 蛇头也在 (5,5)
		IsAlive:  true,
	}
	// 头对头碰撞，双方都应检测到
	if !CheckInterSnakeCollision(s1, s2) {
		t.Errorf("s1 与 s2 头对头，s1 应检测到碰撞")
	}
	if !CheckInterSnakeCollision(s2, s1) {
		t.Errorf("s1 与 s2 头对头，s2 应检测到碰撞")
	}
}

// ---------- 吃食物检测 ----------

func TestCheckEatFood_Eaten(t *testing.T) {
	s := NewSnake(1, "test")
	s.Body[0] = Point{X: 7, Y: 7}
	food := &Food{Position: Point{X: 7, Y: 7}}
	if !CheckEatFood(s, food) {
		t.Errorf("蛇头与食物坐标重合应返回 true")
	}
}

func TestCheckEatFood_NotEaten(t *testing.T) {
	s := NewSnake(1, "test")
	s.Body[0] = Point{X: 7, Y: 7}
	food := &Food{Position: Point{X: 8, Y: 8}}
	if CheckEatFood(s, food) {
		t.Errorf("蛇头与食物坐标不重合应返回 false")
	}
}

// ---------- 食物生成 ----------

func TestGenerateFood_NotOnSnake(t *testing.T) {
	snakes := []*Snake{
		NewSnake(1, "player1"),
		NewSnake(2, "player2"),
	}

	for i := 0; i < 100; i++ {
		food := GenerateFood(snakes, DefaultMapWidth, DefaultMapHeight)
		if food == nil {
			t.Fatal("GenerateFood 不应返回 nil")
		}
		// 检查食物是否与任何蛇身重叠
		for _, snake := range snakes {
			for _, body := range snake.Body {
				if food.Position.X == body.X && food.Position.Y == body.Y {
					t.Errorf("食物 (%d,%d) 不应生成在蛇身上", food.Position.X, food.Position.Y)
				}
			}
		}
		// 检查食物是否在地图范围内
		if food.Position.X < 0 || food.Position.X >= DefaultMapWidth ||
			food.Position.Y < 0 || food.Position.Y >= DefaultMapHeight {
			t.Errorf("食物 (%d,%d) 超出地图范围", food.Position.X, food.Position.Y)
		}
		// 检查默认分值
		if food.ScoreValue != 10 {
			t.Errorf("食物默认分值应为 10，got %d", food.ScoreValue)
		}
	}
}

// ---------- 得分 ----------

func TestSnake_AddScore(t *testing.T) {
	s := NewSnake(1, "test")
	if s.Score != 0 {
		t.Errorf("初始得分应为 0")
	}
	s.AddScore(10)
	if s.Score != 10 {
		t.Errorf("加 10 分后应为 10，got %d", s.Score)
	}
	s.AddScore(5)
	if s.Score != 15 {
		t.Errorf("再加 5 分后应为 15，got %d", s.Score)
	}
}

// ---------- 死亡标记 ----------

func TestSnake_Die(t *testing.T) {
	s := NewSnake(1, "test")
	if !s.IsAlive {
		t.Errorf("新蛇应存活")
	}
	s.Die()
	if s.IsAlive {
		t.Errorf("调用 Die 后 IsAlive 应为 false")
	}
}

// ---------- 移动与碰撞组合场景 ----------

func TestMoveThenCollision(t *testing.T) {
	// 模拟蛇向右移动后撞右墙
	s := &Snake{
		Body:      []Point{{X: 18, Y: 5}, {X: 17, Y: 5}, {X: 16, Y: 5}},
		Direction: DirRight,
	}

	s.Move(false) // 蛇头移动到 (19,5)，仍在边界内
	if CheckWallCollision(s, DefaultMapWidth, DefaultMapHeight) {
		t.Errorf("移动到 (19,5) 不应撞墙")
	}

	s.Move(false) // 蛇头移动到 (20,5)，超出边界
	if !CheckWallCollision(s, DefaultMapWidth, DefaultMapHeight) {
		t.Errorf("移动到 (20,5) 应撞墙")
	}
}

func TestMoveThenEatFood(t *testing.T) {
	// 模拟蛇向右移动后吃到食物
	s := &Snake{
		Body:      []Point{{X: 4, Y: 5}, {X: 3, Y: 5}, {X: 2, Y: 5}},
		Direction: DirRight,
	}

	food := &Food{Position: Point{X: 5, Y: 5}}

	// 移动前长度
	oldLen := len(s.Body)

	// 移动并检测是否吃到食物
	s.Move(false)
	gotEaten := CheckEatFood(s, food)

	if !gotEaten {
		t.Errorf("蛇头移动到 (5,5) 应吃到食物")
	}

	// 吃食物后增长
	s.Move(true)
	if len(s.Body) != oldLen+1 {
		t.Errorf("吃到食物后应增长，got len=%d, want %d", len(s.Body), oldLen+1)
	}
}

// ---------- 不同玩家出生位置不重叠 ----------

func TestSpawnPositions_NotOverlap(t *testing.T) {
	snakes := make([]*Snake, 0, 2)
	for _, pid := range []uint64{1, 2} {
		s := NewSnake(pid, "player")
		snakes = append(snakes, s)
	}

	// 检查两条蛇的蛇身是否重叠
	bodyMap := make(map[Point]int)
	for _, s := range snakes {
		for _, p := range s.Body {
			if owner, exists := bodyMap[p]; exists {
				t.Errorf("玩家 %d 和玩家 %d 的蛇身重叠在 (%d,%d)", owner, s.PlayerID, p.X, p.Y)
			}
			bodyMap[p] = int(s.PlayerID)
		}
	}
}
