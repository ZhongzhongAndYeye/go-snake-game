// 断线测试：验证游戏中断线后游戏是否自动结束
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"
	rpcpb "go-snake-game/pkg/proto/rpc"
)

const (
	serverAddr = "ws://127.0.0.1:8080/ws"
	password   = "123456"
)

type Client struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	msgCh   chan *network.Packet
	stopCh  chan struct{}
}

func newClient() (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(serverAddr, nil)
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn:   conn,
		msgCh:  make(chan *network.Packet, 128),
		stopCh: make(chan struct{}),
	}
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
			select {
			case c.msgCh <- packet:
			default:
			}
		}
	}()
	return c, nil
}

func (c *Client) close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) sendRaw(msgID uint16, seqID uint16, body []byte) {
	data, _ := network.Encode(msgID, seqID, body)
	c.writeMu.Lock()
	_ = c.conn.WriteMessage(websocket.BinaryMessage, data)
	c.writeMu.Unlock()
}

func (c *Client) sendAndReceive(msgID uint16, seqID uint16, body []byte) *network.Packet {
	data, _ := network.Encode(msgID, seqID, body)
	c.writeMu.Lock()
	_ = c.conn.WriteMessage(websocket.BinaryMessage, data)
	c.writeMu.Unlock()
	select {
	case packet := <-c.msgCh:
		return packet
	case <-time.After(10 * time.Second):
		return nil
	}
}

// recvUntil 持续读取消息直到收到指定 MsgID，丢弃中间的其他消息（如推送消息）。
// timeout 内未收到返回 nil。
func (c *Client) recvUntil(expectedMsgID uint16, timeout time.Duration) *network.Packet {
	deadline := time.After(timeout)
	for {
		select {
		case packet, ok := <-c.msgCh:
			if !ok {
				return nil
			}
			if packet.MsgID == expectedMsgID {
				return packet
			}
			// 丢弃其他消息（如推送的 GameStartNotify、GameStateSync 等）
			fmt.Printf("  [recvUntil] 丢弃 MsgID=%d\n", packet.MsgID)
		case <-deadline:
			return nil
		}
	}
}

func decodePacket(data []byte) *network.Packet {
	reader := bufio.NewReader(bytes.NewReader(data))
	packet, err := network.Decode(reader)
	if err != nil {
		return nil
	}
	return packet
}

func registerAndLogin(cl *Client, username, pwd string) {
	req := &msg.RegisterReq{Username: username, Password: pwd}
	body, _ := proto.Marshal(req)
	cl.sendAndReceive(network.MsgIDRegisterReq, 1, body)

	loginReq := &msg.LoginReq{Username: username, Password: pwd}
	loginBody, _ := proto.Marshal(loginReq)
	cl.sendAndReceive(network.MsgIDLoginReq, 2, loginBody)
}

func queryRoomInfo(cl *Client, seqID uint16, roomID string) *rpcpb.GetRoomInfoResponse {
	req := &msg.RoomInfoQueryReq{RoomId: roomID}
	body, _ := proto.Marshal(req)
	cl.sendRaw(network.MsgIDGameRoomInfoReq, seqID, body)
	packet := cl.recvUntil(network.MsgIDGameRoomInfoResp, 5*time.Second)
	if packet == nil {
		return nil
	}
	resp := &rpcpb.GetRoomInfoResponse{}
	_ = proto.Unmarshal(packet.Body, resp)
	return resp
}

func main() {
	fmt.Println("=== 断线测试 ===")

	ts := time.Now().UnixMilli()
	user1 := fmt.Sprintf("dc_1_%d", ts)
	user2 := fmt.Sprintf("dc_2_%d", ts)

	cl1, err := newClient()
	if err != nil {
		fmt.Printf("ERROR: 无法连接 cl1: %v\n", err)
		return
	}
	defer cl1.close()

	cl2, err := newClient()
	if err != nil {
		fmt.Printf("ERROR: 无法连接 cl2: %v\n", err)
		return
	}
	defer cl2.close()

	registerAndLogin(cl1, user1, password)
	registerAndLogin(cl2, user2, password)
	fmt.Println("登录成功")

	// 匹配：cl1 先进入匹配队列
	_ = cl1.sendAndReceive(network.MsgIDMatchStartReq, 1, nil)
	fmt.Println("cl1 已发送匹配请求")

	time.Sleep(100 * time.Millisecond)

	// cl2 发起匹配，使用 recvUntil 过滤推送消息（GameStartNotify 可能先于 MatchStartResp 到达）
	cl2.sendRaw(network.MsgIDMatchStartReq, 1, nil)
	matchResp := cl2.recvUntil(network.MsgIDMatchStartResp, 10*time.Second)
	if matchResp == nil {
		fmt.Println("ERROR: 匹配无响应")
		return
	}

	roomID := ""
	resp := &msg.MatchStartResp{}
	_ = proto.Unmarshal(matchResp.Body, resp)
	roomID = resp.RoomId
	fmt.Printf("匹配成功: room_id=%s, is_matched=%v\n", roomID, resp.IsMatched)

	if roomID == "" {
		fmt.Println("ERROR: roomID 为空")
		return
	}

	// 等待游戏开始
	time.Sleep(500 * time.Millisecond)

	// 查询房间状态（断线前）
	info := queryRoomInfo(cl2, 10, roomID)
	if info != nil {
		fmt.Printf("断线前房间状态: GameStatus=%d, Status=%d, Frame=%d\n", info.GameStatus, info.Status, info.Frame)
		for _, s := range info.Snakes {
			fmt.Printf("  蛇: player_id=%d, alive=%v, score=%d\n", s.PlayerId, s.IsAlive, s.Score)
		}
	}

	// 断开 cl1
	fmt.Println("断开 cl1...")
	cl1.close()
	time.Sleep(500 * time.Millisecond)

	// 等待游戏结束
	fmt.Println("等待游戏结束...")
	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)
		info := queryRoomInfo(cl2, uint16(20+i), roomID)
		if info == nil {
			fmt.Printf("  第%d次查询: info=nil\n", i+1)
			continue
		}
		fmt.Printf("  第%d次查询: GameStatus=%d, Status=%d, Frame=%d\n", i+1, info.GameStatus, info.Status, info.Frame)
		if info.GameStatus == 4 {
			fmt.Println("PASS: 游戏自动结束!")
			return
		}
	}

	fmt.Println("FAIL: 游戏未在30秒内自动结束")
}
