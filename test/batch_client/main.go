// 批量压测客户端
// 启动 N 对玩家同时匹配对战，验证服务端并发稳定性。
//
// 用法：
//
//	go run test/batch_client/main.go [轮数] [每轮对数]
//
// 默认：1 轮，10 对玩家。
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
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

// 推送消息ID集合 — 这些消息不会被投递到 msgCh
var pushMsgIDs = map[uint16]bool{
	network.MsgIDGameStartNotify: true,
	network.MsgIDGameStateSync:   true,
	network.MsgIDGameOverNotify:  true,
}

// ---- 统计计数器 ----

var (
	totalPairs    int32 // 总对局数
	successPairs  int32 // 成功对局数
	failPairs     int32 // 失败对局数
	totalErrors   int32 // 总错误数
	totalDuration int64 // 总耗时（毫秒）
)

// ---- Client ----

// Client 封装一个 WebSocket 连接。
type Client struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex // 保护 WriteMessage 的并发写
	msgCh     chan *network.Packet
	stopCh    chan struct{}
	closeOnce sync.Once
}

func newClient() (*Client, error) {
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
			if pushMsgIDs[packet.MsgID] {
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
	c.closeOnce.Do(func() {
		close(c.stopCh)
		c.conn.Close()
	})
}

func (c *Client) sendAndReceive(msgID uint16, seqID uint16, body []byte) *network.Packet {
	data, err := network.Encode(msgID, seqID, body)
	if err != nil {
		atomic.AddInt32(&totalErrors, 1)
		return nil
	}

	c.writeMu.Lock()
	writeErr := c.conn.WriteMessage(websocket.BinaryMessage, data)
	c.writeMu.Unlock()
	if writeErr != nil {
		atomic.AddInt32(&totalErrors, 1)
		return nil
	}

	select {
	case packet := <-c.msgCh:
		return packet
	case <-time.After(10 * time.Second):
		atomic.AddInt32(&totalErrors, 1)
		return nil
	case <-c.stopCh:
		return nil
	}
}

// ---- 单对玩家对局流程 ----

// pairResult 一对玩家的对局结果
type pairResult struct {
	pairIndex int
	player1ID uint64
	player2ID uint64
	roomID    string
	success   bool
	errs      int
	duration  time.Duration
}

// runPair 运行一对玩家的完整对局流程。
func runPair(pairIndex int, round int, resultCh chan<- pairResult) {
	start := time.Now()
	result := pairResult{pairIndex: pairIndex}

	ts := time.Now().UnixMilli()
	user1 := fmt.Sprintf("b%d_p%d_%d_1", round, pairIndex, ts)
	user2 := fmt.Sprintf("b%d_p%d_%d_2", round, pairIndex, ts)

	// 1. 连接 + 注册 + 登录
	cl1, err := newClient()
	if err != nil {
		result.errs++
		atomic.AddInt32(&failPairs, 1)
		resultCh <- result
		return
	}
	defer cl1.close()

	cl2, err := newClient()
	if err != nil {
		result.errs++
		atomic.AddInt32(&failPairs, 1)
		resultCh <- result
		return
	}
	defer cl2.close()

	seqID := uint16(0)
	seqID++
	if !register(cl1, seqID, user1) {
		result.errs++
		atomic.AddInt32(&failPairs, 1)
		resultCh <- result
		return
	}
	seqID++
	result.player1ID = login(cl1, seqID, user1)
	if result.player1ID == 0 {
		result.errs++
		atomic.AddInt32(&failPairs, 1)
		resultCh <- result
		return
	}

	seqID = 0
	seqID++
	if !register(cl2, seqID, user2) {
		result.errs++
		atomic.AddInt32(&failPairs, 1)
		resultCh <- result
		return
	}
	seqID++
	result.player2ID = login(cl2, seqID, user2)
	if result.player2ID == 0 {
		result.errs++
		atomic.AddInt32(&failPairs, 1)
		resultCh <- result
		return
	}

	// 2. 双方先后发起匹配（错开 50ms，避免同时入队导致互配不到）
	seqID = 0
	seqID++
	roomID1 := matchStart(cl1, seqID)
	time.Sleep(50 * time.Millisecond) // 确保玩家1先入队，玩家2 LPop 到玩家1
	seqID = 0
	seqID++
	roomID2 := matchStart(cl2, seqID)

	if roomID1 != "" {
		result.roomID = roomID1
	}
	if roomID2 != "" {
		result.roomID = roomID2
	}

	if result.roomID == "" {
		result.errs++
		atomic.AddInt32(&failPairs, 1)
		resultCh <- result
		return
	}

	// 3. 等待游戏启动
	time.Sleep(200 * time.Millisecond)

	// 4. 自动发送随机方向操作（模拟正常游戏）
	stopDir := make(chan struct{})
	var dirWg sync.WaitGroup
	dirWg.Add(2)

	go sendRandomDirections(cl1, stopDir, &dirWg)
	go sendRandomDirections(cl2, stopDir, &dirWg)

	// 5. 轮询房间状态，等待游戏结束
	gameEnded := false
	seqID = 0
	for i := 0; i < 150; i++ { // 最多等待 30 秒
		time.Sleep(200 * time.Millisecond)
		seqID++
		roomInfo := queryRoomInfo(cl1, seqID, result.roomID)
		if roomInfo != nil && roomInfo.GameStatus == 4 {
			gameEnded = true
			break
		}
	}

	// 停止方向发送
	close(stopDir)
	dirWg.Wait()

	if !gameEnded {
		atomic.AddInt32(&totalErrors, 1)
	}

	result.success = gameEnded
	result.duration = time.Since(start)

	atomic.AddInt32(&totalPairs, 1)
	if result.success {
		atomic.AddInt32(&successPairs, 1)
	} else {
		atomic.AddInt32(&failPairs, 1)
	}
	atomic.AddInt64(&totalDuration, result.duration.Milliseconds())

	resultCh <- result
}

// ---- 辅助函数 ----

func register(cl *Client, seqID uint16, user string) bool {
	req := &msg.RegisterReq{Username: user, Password: password}
	body, _ := proto.Marshal(req)
	packet := cl.sendAndReceive(network.MsgIDRegisterReq, seqID, body)
	if packet == nil {
		return false
	}
	if packet.MsgID == network.MsgIDRegisterResp {
		resp := &msg.RegisterResp{}
		_ = proto.Unmarshal(packet.Body, resp)
		return resp.Code == 0
	}
	return false
}

func login(cl *Client, seqID uint16, user string) uint64 {
	req := &msg.LoginReq{Username: user, Password: password}
	body, _ := proto.Marshal(req)
	packet := cl.sendAndReceive(network.MsgIDLoginReq, seqID, body)
	if packet == nil {
		return 0
	}
	if packet.MsgID == network.MsgIDLoginResp {
		resp := &msg.LoginResp{}
		_ = proto.Unmarshal(packet.Body, resp)
		if resp.Code == 0 {
			return uint64(resp.PlayerId)
		}
	}
	return 0
}

func matchStart(cl *Client, seqID uint16) string {
	packet := cl.sendAndReceive(network.MsgIDMatchStartReq, seqID, nil)
	if packet == nil {
		return ""
	}
	if packet.MsgID == network.MsgIDMatchStartResp {
		resp := &msg.MatchStartResp{}
		_ = proto.Unmarshal(packet.Body, resp)
		return resp.RoomId
	}
	return ""
}

func sendRandomDirections(cl *Client, stop <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	seqID := uint16(0)
	directions := []int32{1, 2, 3, 4} // 上、下、左、右

	for {
		select {
		case <-stop:
			return
		default:
		}
		time.Sleep(150 * time.Millisecond) // 略高于 100ms 限流阈值

		seqID++
		dir := directions[rand.Intn(4)]
		req := &msg.GameOperationReq{Direction: dir}
		body, _ := proto.Marshal(req)
		// 使用 sendAndReceive，但忽略响应（可能是限流丢弃）
		cl.sendAndReceive(network.MsgIDGameOperationReq, seqID, body)
	}
}

func queryRoomInfo(cl *Client, seqID uint16, roomID string) *rpcpb.GetRoomInfoResponse {
	req := &msg.RoomInfoQueryReq{RoomId: roomID}
	body, _ := proto.Marshal(req)
	packet := cl.sendAndReceive(network.MsgIDGameRoomInfoReq, seqID, body)
	if packet == nil {
		return nil
	}
	if packet.MsgID == network.MsgIDGameRoomInfoResp {
		resp := &rpcpb.GetRoomInfoResponse{}
		_ = proto.Unmarshal(packet.Body, resp)
		return resp
	}
	return nil
}

func decodePacket(data []byte) *network.Packet {
	reader := bufio.NewReader(bytes.NewReader(data))
	packet, err := network.Decode(reader)
	if err != nil {
		return nil
	}
	return packet
}

// ---- 主入口 ----

func main() {
	rounds := 1
	pairsPerRound := 10

	if len(os.Args) > 1 {
		_, _ = fmt.Sscanf(os.Args[1], "%d", &rounds)
	}
	if len(os.Args) > 2 {
		_, _ = fmt.Sscanf(os.Args[2], "%d", &pairsPerRound)
	}

	fmt.Println("================================================")
	fmt.Println("  批量压测客户端")
	fmt.Printf("  轮数: %d  每轮对数: %d  总玩家: %d\n", rounds, pairsPerRound, rounds*pairsPerRound*2)
	fmt.Println("================================================")

	// 捕获 Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for round := 0; round < rounds; round++ {
		select {
		case <-sigCh:
			fmt.Println("\n收到中断信号，退出压测")
			printSummary(round, rounds)
			return
		default:
		}

		roundStart := time.Now()
		fmt.Printf("\n========== 第 %d 轮 ==========\n", round+1)

		// 重置本轮计数器
		atomic.StoreInt32(&totalPairs, 0)
		atomic.StoreInt32(&successPairs, 0)
		atomic.StoreInt32(&failPairs, 0)
		atomic.StoreInt32(&totalErrors, 0)
		atomic.StoreInt64(&totalDuration, 0)

		resultCh := make(chan pairResult, pairsPerRound)
		var wg sync.WaitGroup

		// 并发启动所有对局（每对错开 100ms，避免匹配队列竞态）
		for i := 0; i < pairsPerRound; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				// 按索引错开启动，确保前一对的 A 玩家已入队后，下一对的 A 玩家才入队
				time.Sleep(time.Duration(idx) * 100 * time.Millisecond)
				runPair(idx, round, resultCh)
			}(i)
		}

		// 等待所有对局完成
		go func() {
			wg.Wait()
			close(resultCh)
		}()

		// 收集结果
		for result := range resultCh {
			status := "✅"
			if !result.success {
				status = "❌"
			}
			fmt.Printf("  %s 对局 #%d | room=%s | p1=%d p2=%d | 耗时=%v\n",
				status, result.pairIndex, result.roomID,
				result.player1ID, result.player2ID, result.duration)
		}

		roundDuration := time.Since(roundStart)
		printRoundSummary(round+1, roundDuration)

		// 轮间等待 1 分钟，观察资源回收
		if round < rounds-1 {
			fmt.Println("\n等待 60 秒，观察资源回收...")
			select {
			case <-sigCh:
				fmt.Println("\n收到中断信号，退出压测")
				printSummary(round+1, rounds)
				return
			case <-time.After(60 * time.Second):
			}
		}
	}

	printSummary(rounds, rounds)
}

func printRoundSummary(round int, duration time.Duration) {
	success := atomic.LoadInt32(&successPairs)
	fail := atomic.LoadInt32(&failPairs)
	total := atomic.LoadInt32(&totalPairs)
	errs := atomic.LoadInt32(&totalErrors)
	avgMs := int64(0)
	if total > 0 {
		avgMs = atomic.LoadInt64(&totalDuration) / int64(total)
	}

	fmt.Printf("\n--- 第 %d 轮汇总 ---\n", round)
	fmt.Printf("  总对局: %d  成功: %d  失败: %d\n", total, success, fail)
	fmt.Printf("  总错误: %d\n", errs)
	fmt.Printf("  平均耗时: %dms\n", avgMs)
	fmt.Printf("  本轮耗时: %v\n", duration)
}

func printSummary(completedRounds, totalRounds int) {
	fmt.Println("\n================================================")
	fmt.Printf("  压测结束 (完成 %d/%d 轮)\n", completedRounds, totalRounds)
	fmt.Println("================================================")
}
