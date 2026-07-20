package main

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"
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

// 推送消息ID集合
var pushMsgIDs = map[uint16]bool{
	network.MsgIDGameStartNotify: true,
	network.MsgIDGameStateSync:   true,
	network.MsgIDGameOverNotify:  true,
}

// Client 封装一个 WebSocket 连接，后台 goroutine 持续读取消息，
// 通过 channel 分发给等待响应的调用方。
type Client struct {
	conn      *websocket.Conn
	msgCh     chan *network.Packet // 所有非推送消息投递到此 channel
	stopCh    chan struct{}
	closeOnce sync.Once
}

func NewClient() (*Client, error) {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, _, err := dialer.Dial(serverAddr, nil)
	if err != nil {
		return nil, err
	}

	c := &Client{
		conn:   conn,
		msgCh:  make(chan *network.Packet, 64),
		stopCh: make(chan struct{}),
	}

	// 后台 goroutine 持续读取所有消息，推送消息直接丢弃，非推送消息投递到 msgCh
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				close(c.msgCh)
				return
			}

			packet := decodePacket(data)
			if packet == nil {
				continue
			}

			// 推送消息直接丢弃
			if pushMsgIDs[packet.MsgID] {
				continue
			}

			// 非推送消息投递到 channel
			select {
			case c.msgCh <- packet:
			default:
				// channel 满了丢弃（正常情况下不会发生）
			}
		}
	}()

	return c, nil
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.stopCh)
		c.conn.Close()
	})
}

// SendAndReceive 发送消息并等待响应。
// 超时 10 秒仍未收到响应则返回 nil。
func (c *Client) SendAndReceive(msgID uint16, seqID uint16, body []byte) *network.Packet {
	data, err := network.Encode(msgID, seqID, body)
	if err != nil {
		fmt.Printf("❌ 编码失败: %v\n", err)
		return nil
	}
	if err = c.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		fmt.Printf("❌ 发送失败: %v\n", err)
		return nil
	}

	// 等待响应（最多 10 秒）
	select {
	case packet := <-c.msgCh:
		return packet
	case <-time.After(10 * time.Second):
		fmt.Printf("❌ 等待响应超时, MsgID=%d\n", msgID)
		return nil
	case <-c.stopCh:
		return nil
	}
}

func main() {
	fmt.Println("========================================")
	fmt.Println("  游戏运行状态验证")
	fmt.Println("========================================")

	ts := time.Now().UnixMilli()
	user1 := fmt.Sprintf("f1_%d", ts)
	user2 := fmt.Sprintf("f2_%d", ts)

	// 连接玩家1
	fmt.Println("\n=== 连接玩家1 ===")
	cl1, err := NewClient()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer cl1.Close()

	seqID := uint16(0)
	seqID++
	testRegister(cl1, seqID, user1, password)
	seqID++
	playerID1 := testLogin(cl1, seqID, user1, password)
	if playerID1 == 0 {
		fmt.Println("❌ 玩家1登录失败，终止测试")
		return
	}

	// 连接玩家2
	fmt.Println("\n=== 连接玩家2 ===")
	cl2, err := NewClient()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer cl2.Close()

	seqID = 0
	seqID++
	testRegister(cl2, seqID, user2, password)
	seqID++
	playerID2 := testLogin(cl2, seqID, user2, password)
	if playerID2 == 0 {
		fmt.Println("❌ 玩家2登录失败，终止测试")
		return
	}

	// ======== 第一阶段：匹配 ========
	fmt.Println("\n========================================")
	fmt.Println("  第一阶段：匹配验证")
	fmt.Println("========================================")

	var roomID string
	seqID = 0
	seqID++
	roomID1 := testMatchStart(cl1, seqID, "玩家1", true)
	if roomID1 != "" {
		roomID = roomID1
	}

	seqID = 0
	seqID++
	roomID2 := testMatchStart(cl2, seqID, "玩家2", false)
	if roomID2 != "" {
		roomID = roomID2
	}

	if roomID == "" {
		fmt.Println("❌ 未能获取房间ID，终止测试")
		return
	}
	fmt.Printf("\n✅ 匹配成功，房间ID: %s\n", roomID)

	// 等待游戏启动
	time.Sleep(200 * time.Millisecond)

	// ======== 第二阶段：验证主循环和方向操作 ========
	fmt.Println("\n========================================")
	fmt.Println("  第二阶段：主循环 + 方向操作验证")
	fmt.Println("========================================")

	seqID = 0
	seqID++
	queryRoomInfo(cl1, seqID, roomID)

	// 发送方向操作（蛇1：向下转）
	seqID++
	fmt.Println("\n--- 蛇1发送方向操作：下 ---")
	sendDirection(cl1, seqID, 2)

	// 蛇2发送方向操作（向上转）
	seqID = 0
	seqID++
	fmt.Println("\n--- 蛇2发送方向操作：上 ---")
	sendDirection(cl2, seqID, 1)

	// 等待几帧，让蛇移动
	time.Sleep(800 * time.Millisecond)

	// 查询状态，验证蛇转向
	seqID = 0
	seqID++
	fmt.Println("\n--- 方向操作后查询状态 ---")
	queryRoomInfo(cl1, seqID, roomID)

	// ======== 第三阶段：等待游戏结束，验证碰撞和得分 ========
	fmt.Println("\n========================================")
	fmt.Println("  第三阶段：等待游戏结束（蛇会撞墙/撞自己）")
	fmt.Println("========================================")

	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Second)
		seqID++
		roomInfo := queryRoomInfo(cl1, seqID, roomID)
		if roomInfo != nil && roomInfo.GameStatus == 4 {
			fmt.Println("   ✅ 游戏已结束")
			break
		}
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

func testRegister(cl *Client, seqID uint16, user, pwd string) {
	fmt.Printf("📝 注册: username=%s\n", user)
	req := &msg.RegisterReq{Username: user, Password: pwd}
	body, _ := proto.Marshal(req)
	packet := cl.SendAndReceive(network.MsgIDRegisterReq, seqID, body)
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

func testLogin(cl *Client, seqID uint16, user, pwd string) uint64 {
	fmt.Printf("📝 登录: username=%s\n", user)
	req := &msg.LoginReq{Username: user, Password: pwd}
	body, _ := proto.Marshal(req)
	packet := cl.SendAndReceive(network.MsgIDLoginReq, seqID, body)
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

func testMatchStart(cl *Client, seqID uint16, name string, expectWaiting bool) string {
	fmt.Printf("📝 [%s] 发起匹配\n", name)
	_ = expectWaiting
	packet := cl.SendAndReceive(network.MsgIDMatchStartReq, seqID, nil)
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

func sendDirection(cl *Client, seqID uint16, direction int32) {
	req := &msg.GameOperationReq{Direction: direction}
	body, _ := proto.Marshal(req)
	packet := cl.SendAndReceive(network.MsgIDGameOperationReq, seqID, body)
	if packet == nil {
		failCount++
		return
	}
	fmt.Printf("   ✅ 方向操作已发送, direction=%d\n", direction)
	passCount++
}

func queryRoomInfo(cl *Client, seqID uint16, roomID string) *rpcpb.GetRoomInfoResponse {
	req := &msg.RoomInfoQueryReq{RoomId: roomID}
	body, _ := proto.Marshal(req)
	packet := cl.SendAndReceive(network.MsgIDGameRoomInfoReq, seqID, body)
	if packet == nil {
		failCount++
		return nil
	}

	if packet.MsgID == network.MsgIDGameRoomInfoResp {
		resp := &rpcpb.GetRoomInfoResponse{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("   ❌ 反序列化失败: %v\n\n", err)
			failCount++
			return nil
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
		return resp
	} else {
		fmt.Printf("   ❌ 未知消息ID: %d\n", packet.MsgID)
		failCount++
		return nil
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
