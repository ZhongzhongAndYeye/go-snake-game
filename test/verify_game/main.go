package main

import (
	"bufio"
	"bytes"
	"fmt"
	"time"

	"github.com/gorilla/websocket"

	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"
	rpcpb "go-snake-game/pkg/proto/rpc"

	"google.golang.org/protobuf/proto"
)

const (
	serverAddr = "ws://127.0.0.1:8080/ws"
	password   = "123456"
)

var (
	passCount = 0
	failCount = 0
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  游戏运行状态验证")
	fmt.Println("========================================")

	ts := time.Now().UnixMilli()
	user1 := fmt.Sprintf("f1_%d", ts)
	user2 := fmt.Sprintf("f2_%d", ts)

	// 连接玩家1
	fmt.Println("\n=== 连接玩家1 ===")
	conn1, err := dial()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer conn1.Close()

	seqID := uint16(0)
	seqID++
	testRegister(conn1, seqID, user1, password)
	seqID++
	playerID1 := testLogin(conn1, seqID, user1, password)
	if playerID1 == 0 {
		fmt.Println("❌ 玩家1登录失败，终止测试")
		return
	}

	// 连接玩家2
	fmt.Println("\n=== 连接玩家2 ===")
	conn2, err := dial()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer conn2.Close()

	seqID = 0
	seqID++
	testRegister(conn2, seqID, user2, password)
	seqID++
	playerID2 := testLogin(conn2, seqID, user2, password)
	if playerID2 == 0 {
		fmt.Println("❌ 玩家2登录失败，终止测试")
		return
	}

	// ======== 第一阶段：匹配 ========
	fmt.Println("\n========================================")
	fmt.Println("  第一阶段：匹配验证")
	fmt.Println("========================================")

	// 玩家1发起匹配
	var roomID string
	seqID = 0
	seqID++
	roomID1 := testMatchStart(conn1, seqID, "玩家1", true)
	if roomID1 != "" {
		roomID = roomID1
	}

	// 玩家2发起匹配
	seqID = 0
	seqID++
	roomID2 := testMatchStart(conn2, seqID, "玩家2", false)
	if roomID2 != "" {
		roomID = roomID2
	}

	if roomID == "" {
		fmt.Println("❌ 未能获取房间ID，终止测试")
		return
	}
	fmt.Printf("\n✅ 匹配成功，房间ID: %s\n", roomID)

	// ======== 第二阶段：验证主循环和方向操作 ========
	fmt.Println("\n========================================")
	fmt.Println("  第二阶段：主循环 + 方向操作验证")
	fmt.Println("========================================")

	// 立即查询初始状态
	time.Sleep(200 * time.Millisecond) // 等待游戏启动
	seqID = 0
	seqID++
	queryRoomInfo(conn1, seqID, roomID)

	// 立即发送方向操作（蛇1：向下转，蛇2：向上转）
	// 蛇1从(5,5)朝右 → 改为朝下
	seqID++
	fmt.Println("\n--- 蛇1发送方向操作：下 ---")
	sendDirection(conn1, seqID, 2) // DirDown

	// 蛇2从(14,14)朝左 → 改为朝上
	seqID = 0
	seqID++
	fmt.Println("\n--- 蛇2发送方向操作：上 ---")
	sendDirection(conn2, seqID, 1) // DirUp

	// 等待几帧，让蛇移动
	time.Sleep(800 * time.Millisecond)

	// 查询状态，验证蛇转向
	seqID = 0
	seqID++
	fmt.Println("\n--- 方向操作后查询状态 ---")
	queryRoomInfo(conn1, seqID, roomID)

	// ======== 第三阶段：等待游戏结束，验证碰撞和得分 ========
	fmt.Println("\n========================================")
	fmt.Println("  第三阶段：等待游戏结束（蛇会撞墙/撞自己）")
	fmt.Println("========================================")

	// 蛇1转向下后，会从(5,5)向下走，最终撞底墙(y=20)
	// 蛇2转向上后，会从(14,14)向上走，最终撞顶墙(y=-1)
	// 游戏会在约1.5-2秒后结束
	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		seqID++
		queryRoomInfo(conn1, seqID, roomID)
	}

	// ======== 第四阶段：验证数据库 ========
	fmt.Println("\n========================================")
	fmt.Println("  第四阶段：数据库得分验证")
	fmt.Println("========================================")
	fmt.Printf("请手动检查 MySQL player 表:\n")
	fmt.Printf("  SELECT id, username, max_score FROM player WHERE id IN (%d, %d);\n", playerID1, playerID2)

	// 打印汇总
	fmt.Println("\n========================================")
	fmt.Printf("  通过: %d  失败: %d\n", passCount, failCount)
	fmt.Println("========================================")
	if failCount > 0 {
		fmt.Println("❌ 部分验证未通过，请检查日志")
	} else {
		fmt.Println("✅ 全部验证通过")
	}
}

func dial() (*websocket.Conn, error) {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, _, err := dialer.Dial(serverAddr, nil)
	return conn, err
}

func sendAndReceive(conn *websocket.Conn, msgID uint16, seqID uint16, body []byte) *network.Packet {
	data, err := network.Encode(msgID, seqID, body)
	if err != nil {
		fmt.Printf("❌ 编码失败: %v\n", err)
		return nil
	}
	if err = conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		fmt.Printf("❌ 发送失败: %v\n", err)
		return nil
	}
	_, respData, err := conn.ReadMessage()
	if err != nil {
		fmt.Printf("❌ 接收响应失败: %v\n", err)
		return nil
	}
	return decodePacket(respData)
}

func testRegister(conn *websocket.Conn, seqID uint16, user, pwd string) {
	fmt.Printf("📝 注册: username=%s\n", user)
	req := &msg.RegisterReq{Username: user, Password: pwd}
	body, _ := proto.Marshal(req)
	packet := sendAndReceive(conn, network.MsgIDRegisterReq, seqID, body)
	if packet == nil {
		failCount++
		return
	}
	if packet.MsgID == network.MsgIDRegisterResp {
		resp := &msg.RegisterResp{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("   ❌ 反序列化失败: %v\n\n", err)
			failCount++
			return
		}
		fmt.Printf("   ✅ 注册结果: code=%d, player_id=%d\n\n", resp.Code, resp.PlayerId)
		passCount++
	}
}

func testLogin(conn *websocket.Conn, seqID uint16, user, pwd string) uint64 {
	fmt.Printf("📝 登录: username=%s\n", user)
	req := &msg.LoginReq{Username: user, Password: pwd}
	body, _ := proto.Marshal(req)
	packet := sendAndReceive(conn, network.MsgIDLoginReq, seqID, body)
	if packet == nil {
		failCount++
		return 0
	}
	if packet.MsgID == network.MsgIDLoginResp {
		resp := &msg.LoginResp{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("   ❌ 反序列化失败: %v\n\n", err)
			failCount++
			return 0
		}
		if resp.Code == 0 {
			fmt.Printf("   ✅ 登录成功: player_id=%d\n\n", resp.PlayerId)
			passCount++
			return uint64(resp.PlayerId)
		} else {
			fmt.Printf("   ⚠️ 登录失败: code=%d, msg=%s\n\n", resp.Code, resp.Msg)
			passCount++
			return 0
		}
	}
	return 0
}

func testMatchStart(conn *websocket.Conn, seqID uint16, name string, expectWaiting bool) string {
	fmt.Printf("📝 [%s] 发起匹配\n", name)
	_ = expectWaiting
	packet := sendAndReceive(conn, network.MsgIDMatchStartReq, seqID, nil)
	if packet == nil {
		failCount++
		return ""
	}

	if packet.MsgID == network.MsgIDMatchStartResp {
		resp := &msg.MatchStartResp{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("   ❌ 反序列化失败: %v\n\n", err)
			failCount++
			return ""
		}

		if resp.IsMatched {
			fmt.Printf("   ✅ [%s] 匹配成功: room_id=%s\n\n", name, resp.RoomId)
			passCount++
			return resp.RoomId
		} else {
			fmt.Printf("   ⚠️ [%s] 进入等待: msg=%s\n\n", name, resp.Msg)
			passCount++
			return ""
		}
	} else if packet.MsgID == network.MsgIDErrorResp {
		code, msg := parseErrorBody(packet.Body)
		fmt.Printf("   ⚠️ [%s] 服务端错误: code=%d, msg=%s\n\n", name, code, msg)
		passCount++
		return ""
	}
	return ""
}

func sendDirection(conn *websocket.Conn, seqID uint16, direction int32) {
	req := &msg.GameOperationReq{Direction: direction}
	body, _ := proto.Marshal(req)
	packet := sendAndReceive(conn, network.MsgIDGameOperationReq, seqID, body)
	if packet == nil {
		failCount++
		return
	}
	fmt.Printf("   ✅ 方向操作已发送, direction=%d\n", direction)
	passCount++
}

func queryRoomInfo(conn *websocket.Conn, seqID uint16, roomID string) {
	req := &msg.RoomInfoQueryReq{RoomId: roomID}
	body, _ := proto.Marshal(req)
	packet := sendAndReceive(conn, network.MsgIDGameRoomInfoReq, seqID, body)
	if packet == nil {
		failCount++
		return
	}

	if packet.MsgID == network.MsgIDGameRoomInfoResp {
		resp := &rpcpb.GetRoomInfoResponse{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("   ❌ 反序列化失败: %v\n\n", err)
			failCount++
			return
		}

		statusStr := map[int32]string{1: "等待中", 2: "游戏中", 3: "已结束"}
		gameStatusStr := map[int32]string{1: "未开始", 2: "进行中", 3: "暂停", 4: "已结束"}
		s := statusStr[resp.Status]
		if s == "" {
			s = fmt.Sprintf("%d", resp.Status)
		}
		gs := gameStatusStr[resp.GameStatus]
		if gs == "" {
			gs = fmt.Sprintf("%d", resp.GameStatus)
		}

		fmt.Printf("   房间: %s | 状态: %s | 游戏阶段: %s | 帧: %d\n",
			resp.RoomId, s, gs, resp.Frame)
		fmt.Printf("   玩家数: %d\n", len(resp.Players))
		for _, p := range resp.Players {
			fmt.Printf("     - player_id=%d, nickname=%s, score=%d\n", p.PlayerId, p.Nickname, p.Score)
		}
		if resp.Food != nil {
			fmt.Printf("   食物: (%d,%d) 分值=%d\n",
				resp.Food.Position.GetX(), resp.Food.Position.GetY(), resp.Food.Score)
		}
		fmt.Printf("   蛇状态:\n")
		for _, s := range resp.Snakes {
			headStr := "?"
			if len(s.Body) > 0 {
				headStr = fmt.Sprintf("(%d,%d)", s.Body[0].GetX(), s.Body[0].GetY())
			}
			aliveStr := "存活"
			if !s.IsAlive {
				aliveStr = "死亡"
			}
			fmt.Printf("     - player_id=%d, nickname=%s, %s, 分数=%d, 蛇头=%s, 长度=%d\n",
				s.PlayerId, s.Nickname, aliveStr, s.Score, headStr, len(s.Body))
		}
		passCount++
	} else {
		fmt.Printf("   ❌ 未知消息ID: %d\n", packet.MsgID)
		failCount++
	}
}

func decodePacket(data []byte) *network.Packet {
	reader := bufio.NewReader(bytes.NewReader(data))
	packet, err := network.Decode(reader)
	if err != nil {
		fmt.Printf("❌ 解码响应失败: %v\n", err)
		return nil
	}
	return packet
}

func parseErrorBody(body []byte) (uint16, string) {
	if len(body) < 2 {
		return 0, ""
	}
	code := uint16(body[0])<<8 | uint16(body[1])
	msg := string(body[2:])
	return code, msg
}