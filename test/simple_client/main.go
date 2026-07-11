package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go-snake-game/pkg/network"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var (
	addr      = flag.String("addr", "127.0.0.1:8080", "服务端地址")
	connCount = flag.Int("n", 1, "连接数")
)

func main() {
	flag.Parse()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var conns []*websocket.Conn

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	for i := 0; i < *connCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			url := fmt.Sprintf("ws://%s/ws", *addr)
			conn, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				log.Printf("[连接-%d] 连接失败: %v", id, err)
				return
			}

			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()

			log.Printf("[连接-%d] 连接成功", id)

			defer func() {
				mu.Lock()
				for j, c := range conns {
					if c == conn {
						conns = append(conns[:j], conns[j+1:]...)
						break
					}
				}
				mu.Unlock()
				conn.Close()
				log.Printf("[连接-%d] 连接已关闭", id)
			}()

			done := make(chan struct{})

			wg.Add(1)
			go func() {
				defer wg.Done()
				defer close(done)

				for {
					_, message, err := conn.ReadMessage()
					if err != nil {
						log.Printf("[连接-%d] 读取消息失败: %v", id, err)
						return
					}

					reader := bufio.NewReader(bytes.NewReader(message))
					pkt, err := network.Decode(reader)
					if err != nil {
						log.Printf("[连接-%d] 解码消息失败: %v", id, err)
						continue
					}

					log.Printf("[连接-%d] 收到服务端响应: msgID=%d, seqID=%d, body_len=%d",
						id, pkt.MsgID, pkt.SeqID, len(pkt.Body))
				}
			}()

			heartbeatTicker := time.NewTicker(10 * time.Second)
			defer heartbeatTicker.Stop()

			var seqID uint16 = 0

			for {
				select {
				case <-heartbeatTicker.C:
					seqID++
					// data, err := network.Encode(network.MsgIDHeartbeatReq, seqID, nil)

					// 测试 发一条不存在的消息id，看会不会报错
					data, err := network.Encode(12138, seqID, nil)
					if err != nil {
						log.Printf("[连接-%d] 编码心跳消息失败: %v", id, err)
						continue
					}

					err = conn.WriteMessage(websocket.BinaryMessage, data)
					if err != nil {
						log.Printf("[连接-%d] 发送心跳失败: %v", id, err)
						return
					}
					log.Printf("[连接-%d] 发送心跳请求: msgID=%d, seqID=%d",
						id, network.MsgIDHeartbeatReq, seqID)

				case <-done:
					log.Printf("[连接-%d] 读goroutine退出，停止发送心跳", id)
					return
				}
			}
		}(i + 1)
	}

	log.Printf("已建立 %d 个连接，按 Ctrl+C 退出", *connCount)

	<-interrupt

	log.Println("正在关闭所有连接...")
	mu.Lock()
	for _, conn := range conns {
		conn.Close()
	}
	mu.Unlock()

	wg.Wait()
	log.Println("所有连接已关闭")
}
