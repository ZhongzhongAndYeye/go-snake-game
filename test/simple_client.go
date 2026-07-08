package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
)

var (
	addr = flag.String("addr", "127.0.0.1:8080", "服务端地址")

	// 添加命令行 -n后是连接数设置 不写默认为1
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
				// 删除连接组中的当前连接
				for j, c := range conns {
					if c == conn {
						conns = append(conns[:j], conns[j+1:]...)
						break
					}
				}
				mu.Unlock()

				// 双保险，若是主函数已经关闭一次了，关闭函数的幂等的，重复关闭也不会报错
				// 若是是因为conn网络异常等其他原因导致的ReadMessage出错，这里也可以优雅的关闭连接
				conn.Close()
				log.Printf("[连接-%d] 连接已关闭", id)
			}()

			for {
				// 主函数关闭连接后，ReadMessage会返回错误，这里就会return
				_, message, err := conn.ReadMessage()
				if err != nil {
					log.Printf("[连接-%d] 读取消息失败: %v", id, err)
					return
				}
				log.Printf("[连接-%d] 收到消息: %d 字节", id, len(message))
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

	wg.Wait() // 所有协程结束前，阻塞在此，防止连接还没关完主函数就结束了
	log.Println("所有连接已关闭")
}
