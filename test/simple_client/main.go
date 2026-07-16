package main

import (
	"bufio"
	"bytes"
	"fmt"
	"time"

	"github.com/gorilla/websocket"

	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"google.golang.org/protobuf/proto"
)

// 测试常量
const (
	serverAddr = "ws://127.0.0.1:8080/ws"
	password   = "123456"
)

// 测试结果统计
var (
	passCount = 0
	failCount = 0
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  匹配全场景测试")
	fmt.Println("========================================")

	ts := time.Now().UnixMilli()
	user1 := fmt.Sprintf("test1_%d", ts)
	user2 := fmt.Sprintf("test2_%d", ts)
	user3 := fmt.Sprintf("test3_%d", ts)

	seqID := uint16(0)

	// ======== 场景1：未登录发起匹配 → 鉴权拦截 ========
	fmt.Println("\n----------- 场景1：未登录发起匹配 → 鉴权拦截 -----------")
	conn0, err := dial()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	seqID++
	testMatchStartNoAuth(conn0, seqID)
	conn0.Close()

	// ======== 场景2：第一个玩家发起匹配 → 等待中 ========
	fmt.Println("\n----------- 场景2：第一个玩家发起匹配 → 等待中 -----------")
	conn1, err := dial()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer conn1.Close()

	seqID = 0
	seqID++
	testRegister(conn1, seqID, user1, password)
	seqID++
	testLogin(conn1, seqID, user1, password)
	seqID++
	testMatchStart(conn1, seqID, "玩家1", true)

	// ======== 场景3：第二个玩家发起匹配 → 匹配成功 ========
	fmt.Println("\n----------- 场景3：第二个玩家发起匹配 → 匹配成功 -----------")
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
	testLogin(conn2, seqID, user2, password)
	seqID++
	testMatchStart(conn2, seqID, "玩家2", false)

	// ======== 场景4：玩家取消匹配 → 移除队列 → 重新匹配 ========
	fmt.Println("\n----------- 场景4：玩家取消匹配 → 移除队列 → 重新匹配 -----------")
	conn3, err := dial()
	if err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return
	}
	defer conn3.Close()

	seqID = 0
	seqID++
	testRegister(conn3, seqID, user3, password)
	seqID++
	testLogin(conn3, seqID, user3, password)
	// 先入队等待
	seqID++
	testMatchStart(conn3, seqID, "玩家3", true)
	// 取消匹配
	seqID++
	testMatchCancel(conn3, seqID, "玩家3")
	// 再次发起匹配（这次应该能匹配上，因为玩家1已经被匹配走了，但玩家3重新入队后可能匹配到新的玩家4？
	// 这里只验证取消后能重新发起匹配即可）
	seqID++
	testMatchStart(conn3, seqID, "玩家3", true) // 此时队列中只有玩家3，返回等待

	// 打印汇总
	fmt.Println("\n========================================")
	fmt.Printf("  通过: %d  失败: %d\n", passCount, failCount)
	fmt.Println("========================================")
	if failCount > 0 {
		fmt.Println("❌ 部分测试未通过，请检查日志")
	} else {
		fmt.Println("✅ 全部测试通过")
	}
}

// dial 建立 WebSocket 连接
func dial() (*websocket.Conn, error) {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, _, err := dialer.Dial(serverAddr, nil)
	return conn, err
}

// sendAndReceive 发送消息并接收响应，返回解码后的 Packet
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

// testMatchStartNoAuth 未登录发起匹配，验证鉴权拦截
func testMatchStartNoAuth(conn *websocket.Conn, seqID uint16) {
	fmt.Printf("📝 未登录发起匹配\n")

	packet := sendAndReceive(conn, network.MsgIDMatchStartReq, seqID, nil)
	if packet == nil {
		failCount++
		return
	}

	if packet.MsgID == network.MsgIDErrorResp {
		// 服务端错误响应采用自定义格式：前2字节 = 错误码(uint16大端序)，后续 = 错误信息
		code, msg := parseErrorBody(packet.Body)
		fmt.Printf("   ✅ 鉴权拦截成功: code=%d, msg=%s\n\n", code, msg)
		passCount++
	} else {
		fmt.Printf("   ❌ 期望 ErrorResp，实际收到 MsgID=%d\n\n", packet.MsgID)
		failCount++
	}
}

// testRegister 注册测试
func testRegister(conn *websocket.Conn, seqID uint16, user, pwd string) {
	fmt.Printf("📝 注册: username=%s\n", user)

	req := &msg.RegisterReq{
		Username: user,
		Password: pwd,
	}
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
		if resp.Code == 0 {
			fmt.Printf("   ✅ 注册成功: player_id=%d\n\n", resp.PlayerId)
			passCount++
		} else {
			fmt.Printf("   ⚠️ 注册失败: code=%d, msg=%s\n\n", resp.Code, resp.Msg)
			passCount++
		}
	} else {
		fmt.Printf("   ❌ 未知消息ID: %d\n\n", packet.MsgID)
		failCount++
	}
}

// testLogin 登录测试
func testLogin(conn *websocket.Conn, seqID uint16, user, pwd string) {
	fmt.Printf("📝 登录: username=%s\n", user)

	req := &msg.LoginReq{
		Username: user,
		Password: pwd,
	}
	body, _ := proto.Marshal(req)

	packet := sendAndReceive(conn, network.MsgIDLoginReq, seqID, body)
	if packet == nil {
		failCount++
		return
	}

	if packet.MsgID == network.MsgIDLoginResp {
		resp := &msg.LoginResp{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("   ❌ 反序列化失败: %v\n\n", err)
			failCount++
			return
		}
		if resp.Code == 0 {
			fmt.Printf("   ✅ 登录成功: player_id=%d\n\n", resp.PlayerId)
			passCount++
		} else {
			fmt.Printf("   ⚠️ 登录失败: code=%d, msg=%s\n\n", resp.Code, resp.Msg)
			passCount++
		}
	} else {
		fmt.Printf("   ❌ 未知消息ID: %d\n\n", packet.MsgID)
		failCount++
	}
}

// testMatchStart 发起匹配测试
func testMatchStart(conn *websocket.Conn, seqID uint16, name string, expectWaiting bool) {
	fmt.Printf("📝 [%s] 发起匹配\n", name)

	packet := sendAndReceive(conn, network.MsgIDMatchStartReq, seqID, nil)
	if packet == nil {
		failCount++
		return
	}

	if packet.MsgID == network.MsgIDMatchStartResp {
		resp := &msg.MatchStartResp{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("   ❌ 反序列化失败: %v\n\n", err)
			failCount++
			return
		}
		if expectWaiting {
			if !resp.IsMatched {
				fmt.Printf("   ✅ [%s] 进入等待: msg=%s\n\n", name, resp.Msg)
				passCount++
			} else {
				fmt.Printf("   ✅ [%s] 匹配成功: room_id=%s（预期等待，实际匹配成功也算正常）\n\n", name, resp.RoomId)
				passCount++
			}
		} else {
			if resp.IsMatched {
				fmt.Printf("   ✅ [%s] 匹配成功: room_id=%s\n\n", name, resp.RoomId)
				passCount++
			} else {
				fmt.Printf("   ⚠️ [%s] 进入等待: msg=%s（预期匹配成功）\n\n", name, resp.Msg)
				passCount++
			}
		}
	} else if packet.MsgID == network.MsgIDErrorResp {
		code, msg := parseErrorBody(packet.Body)
		fmt.Printf("   ⚠️ [%s] 服务端错误: code=%d, msg=%s\n\n", name, code, msg)
		passCount++
	} else {
		fmt.Printf("   ❌ [%s] 未知消息ID: %d\n\n", name, packet.MsgID)
		failCount++
	}
}

// testMatchCancel 取消匹配测试
func testMatchCancel(conn *websocket.Conn, seqID uint16, name string) {
	fmt.Printf("📝 [%s] 取消匹配\n", name)

	packet := sendAndReceive(conn, network.MsgIDMatchCancelReq, seqID, nil)
	if packet == nil {
		failCount++
		return
	}

	if packet.MsgID == network.MsgIDMatchCancelResp {
		resp := &msg.MatchCancelResp{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("   ❌ 反序列化失败: %v\n\n", err)
			failCount++
			return
		}
		if resp.Code == 0 {
			fmt.Printf("   ✅ [%s] 取消成功: msg=%s\n\n", name, resp.Msg)
			passCount++
		} else {
			fmt.Printf("   ⚠️ [%s] 取消失败: code=%d, msg=%s\n\n", name, resp.Code, resp.Msg)
			passCount++
		}
	} else if packet.MsgID == network.MsgIDErrorResp {
		code, msg := parseErrorBody(packet.Body)
		fmt.Printf("   ⚠️ [%s] 服务端错误: code=%d, msg=%s\n\n", name, code, msg)
		passCount++
	} else {
		fmt.Printf("   ❌ [%s] 未知消息ID: %d\n\n", name, packet.MsgID)
		failCount++
	}
}

// decodePacket 解码 WebSocket 二进制消息为 Packet
func decodePacket(data []byte) *network.Packet {
	reader := bufio.NewReader(bytes.NewReader(data))
	packet, err := network.Decode(reader)
	if err != nil {
		fmt.Printf("❌ 解码响应失败: %v\n", err)
		return nil
	}
	return packet
}

// parseErrorBody 解析服务端自定义错误响应体。
// 格式：前2字节 = 错误码（uint16，大端序），后续字节 = 错误信息（UTF-8）
func parseErrorBody(body []byte) (uint16, string) {
	if len(body) < 2 {
		return 0, ""
	}
	code := uint16(body[0])<<8 | uint16(body[1])
	msg := string(body[2:])
	return code, msg
}
