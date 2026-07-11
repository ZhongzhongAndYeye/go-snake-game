package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// 统计信息
type Stats struct {
	connected    atomic.Int64 // 连接成功数
	failed       atomic.Int64 // 连接失败数
	disconnected atomic.Int64 // 断开数（已成功连接后断开）
}

func main() {
	// 解析命令行参数
	n := flag.Int("n", 100, "连接数量")
	addr := flag.String("addr", "ws://127.0.0.1:8080/ws", "服务端 WebSocket 地址")
	flag.Parse()

	fmt.Printf("批量连接测试开始：目标 %s，连接数 %d\n", *addr, *n)
	fmt.Println("按 Ctrl+C 退出")

	var stats Stats
	var wg sync.WaitGroup

	// 存储所有连接，用于退出时关闭
	var conns sync.Map

	// 每秒打印统计信息
	stopPrint := make(chan struct{})
	go printStats(&stats, stopPrint)

	// 启动所有连接
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			connectAndRun(id, *addr, &stats, &conns)
		}(i + 1)
	}

	// 等待 Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\n正在退出，请等待所有连接关闭...")

	// 退出：关闭统计打印
	close(stopPrint)

	// 关闭所有 WebSocket 连接，让 readLoop 退出
	conns.Range(func(key, val interface{}) bool {
		conn := val.(*websocket.Conn)
		conn.Close()
		return true
	})

	// 等待所有 goroutine 退出
	wg.Wait()
	printFinalStats(&stats)
}

// connectAndRun 建立连接并维持心跳
func connectAndRun(id int, addr string, stats *Stats, conns *sync.Map) {
	conn, err := dialWithRetry(id, addr)
	if err != nil {
		stats.failed.Add(1)
		fmt.Printf("[%d] 连接失败（重试后）：%v\n", id, err)
		return
	}
	stats.connected.Add(1)

	// 注册连接到全局管理
	conns.Store(id, conn)

	// 启动心跳发送
	stopHeartbeat := make(chan struct{})
	go sendHeartbeat(id, conn, stopHeartbeat)

	// 读取响应，检测连接断开
	readLoop(id, conn, stats)

	// 连接断开，清理
	close(stopHeartbeat)
	conn.Close()
	conns.Delete(id)
	stats.disconnected.Add(1)
}

// dialWithRetry 尝试连接，失败后重试 1 次
func dialWithRetry(id int, addr string) (*websocket.Conn, error) {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second

	conn, _, err := dialer.Dial(addr, nil)
	if err == nil {
		return conn, nil
	}

	// 第一次失败，日志记录
	fmt.Printf("[%d] 首次连接失败，1 秒后重试：%v\n", id, err)

	// 等待 1 秒后重试
	time.Sleep(time.Second)

	conn, _, err = dialer.Dial(addr, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// sendHeartbeat 定时发送心跳消息
func sendHeartbeat(id int, conn *websocket.Conn, stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 等待 1 秒后开始发送心跳，让连接先稳定
	select {
	case <-stop:
		return
	case <-time.After(time.Second):
	}

	for {
		select {
		case <-ticker.C:
			// 不发心跳 看看会不会触发服务端心跳超时断
			// ======================================================================================
			// 使用 network.Encode 编码心跳消息（MsgID=1001，包体为空）
			// data, err := network.Encode(network.MsgIDHeartbeatReq, 0, nil)
			// if err != nil {
			// 	fmt.Printf("[%d] 编码心跳消息失败：%v\n", id, err)
			// 	continue
			// }

			// err = conn.WriteMessage(websocket.BinaryMessage, data)
			// if err != nil {
			// 	fmt.Printf("[%d] 发送心跳失败：%v\n", id, err)
			// 	return
			// }
		case <-stop:
			return
		}
	}
}

// readLoop 读取服务端消息，检测连接断开
func readLoop(id int, conn *websocket.Conn, stats *Stats) {
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			return
		}
	}
}

// printStats 每秒打印统计信息
func printStats(stats *Stats, stop <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			connected := stats.connected.Load()
			failed := stats.failed.Load()
			disconnected := stats.disconnected.Load()
			online := connected - disconnected
			fmt.Printf(" 在线: %d  成功: %d  失败: %d  断开: %d\n",
				online, connected, failed, disconnected)
		case <-stop:
			return
		}
	}
}

// printFinalStats 打印最终统计结果
func printFinalStats(stats *Stats) {
	connected := stats.connected.Load()
	failed := stats.failed.Load()
	disconnected := stats.disconnected.Load()
	online := connected - disconnected

	fmt.Println("\n========== 批量连接测试结果 ==========")
	fmt.Printf("  总连接数:  %d\n", connected+failed)
	fmt.Printf("  连接成功:  %d\n", connected)
	fmt.Printf("  连接失败:  %d\n", failed)
	fmt.Printf("  已断开:    %d\n", disconnected)
	fmt.Printf("  当前在线:  %d\n", online)
	fmt.Println("======================================")
}
