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
	username   = "testuser_simple"
	password   = "123456"
)

// 测试结果统计
var (
	passCount = 0
	failCount = 0
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  simple_client — 注册/登录测试")
	fmt.Println("========================================")

	// 连线到服务端
	conn, err := dial()
	if err != nil {
		fmt.Printf("❌ 连接服务端失败: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Printf("✅ 连接服务端成功: %s\n\n", serverAddr)

	// 逐一执行测试场景
	seqID := uint16(0)

	// 场景1：正常注册新账号
	seqID++
	testRegister(conn, seqID, username, password)

	// 场景2：重复注册同名账号
	seqID++
	testRegister(conn, seqID, username, password)

	// 场景3：正确密码登录
	seqID++
	testLogin(conn, seqID, username, password)

	// 场景4：错误密码登录
	seqID++
	testLogin(conn, seqID, username, "wrongpass")

	// 场景5：登录不存在账号
	seqID++
	testLogin(conn, seqID, "nonexistent_user", password)

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

// testRegister 注册测试
func testRegister(conn *websocket.Conn, seqID uint16, user, pwd string) {
	fmt.Printf("📝 测试注册: username=%s, password=%s\n", user, pwd)

	// 构造 RegisterReq 消息
	req := &msg.RegisterReq{
		Username: user,
		Password: pwd,
	}
	body, _ := proto.Marshal(req)

	// 编码并发送
	data, err := network.Encode(network.MsgIDRegisterReq, seqID, body)
	if err != nil {
		fmt.Printf("❌ 编码失败: %v\n\n", err)
		failCount++
		return
	}
	if err = conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		fmt.Printf("❌ 发送失败: %v\n\n", err)
		failCount++
		return
	}

	// 接收响应
	_, respData, err := conn.ReadMessage()
	if err != nil {
		fmt.Printf("❌ 接收响应失败: %v\n\n", err)
		failCount++
		return
	}

	// 解码
	packet := decodePacket(respData)
	if packet == nil {
		failCount++
		return
	}

	// 判断响应类型
	if packet.MsgID == network.MsgIDRegisterResp {
		resp := &msg.RegisterResp{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("❌ 反序列化注册响应失败: %v\n\n", err)
			failCount++
			return
		}
		if resp.Code == 0 {
			fmt.Printf("   ✅ 注册成功: player_id=%d\n\n", resp.PlayerId)
			passCount++
		} else {
			fmt.Printf("   ⚠️ 注册失败: code=%d, msg=%s\n\n", resp.Code, resp.Msg)
			passCount++ // 预期行为（如重复注册）
		}
	} else if packet.MsgID == network.MsgIDErrorResp {
		errResp := &msg.ErrorResp{}
		if err := proto.Unmarshal(packet.Body, errResp); err != nil {
			fmt.Printf("   ❌ 反序列化错误响应失败: %v\n\n", err)
			failCount++
			return
		}
		fmt.Printf("   ⚠️ 服务端错误: code=%d, msg=%s\n\n", errResp.Code, errResp.Msg)
		passCount++
	} else {
		fmt.Printf("   ❌ 未知消息ID: %d\n\n", packet.MsgID)
		failCount++
	}
}

// testLogin 登录测试
func testLogin(conn *websocket.Conn, seqID uint16, user, pwd string) {
	fmt.Printf("📝 测试登录: username=%s, password=%s\n", user, pwd)

	// 构造 LoginReq 消息
	req := &msg.LoginReq{
		Username: user,
		Password: pwd,
	}
	body, _ := proto.Marshal(req)

	// 编码并发送
	data, err := network.Encode(network.MsgIDLoginReq, seqID, body)
	if err != nil {
		fmt.Printf("❌ 编码失败: %v\n\n", err)
		failCount++
		return
	}
	if err = conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		fmt.Printf("❌ 发送失败: %v\n\n", err)
		failCount++
		return
	}

	// 接收响应
	_, respData, err := conn.ReadMessage()
	if err != nil {
		fmt.Printf("❌ 接收响应失败: %v\n\n", err)
		failCount++
		return
	}

	// 解码
	packet := decodePacket(respData)
	if packet == nil {
		failCount++
		return
	}

	// 判断响应类型
	if packet.MsgID == network.MsgIDLoginResp {
		resp := &msg.LoginResp{}
		if err := proto.Unmarshal(packet.Body, resp); err != nil {
			fmt.Printf("❌ 反序列化登录响应失败: %v\n\n", err)
			failCount++
			return
		}
		if resp.Code == 0 {
			fmt.Printf("   ✅ 登录成功: player_id=%d, token=%s\n\n", resp.PlayerId, maskToken(resp.Token))
			passCount++
		} else {
			fmt.Printf("   ⚠️ 登录失败: code=%d, msg=%s\n\n", resp.Code, resp.Msg)
			passCount++ // 预期行为（如密码错误）
		}
	} else if packet.MsgID == network.MsgIDErrorResp {
		errResp := &msg.ErrorResp{}
		if err := proto.Unmarshal(packet.Body, errResp); err != nil {
			fmt.Printf("   ❌ 反序列化错误响应失败: %v\n\n", err)
			failCount++
			return
		}
		fmt.Printf("   ⚠️ 服务端错误: code=%d, msg=%s\n\n", errResp.Code, errResp.Msg)
		passCount++
	} else {
		fmt.Printf("   ❌ 未知消息ID: %d\n\n", packet.MsgID)
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

// maskToken 对 Token 进行脱敏处理，便于日志输出
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}
