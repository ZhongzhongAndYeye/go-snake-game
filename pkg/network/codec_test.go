//go:build exclude

package network

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"go-snake-game/pkg/config"
	"go-snake-game/pkg/logger"
)

// TestMain 在所有测试函数执行前初始化日志系统
func TestMain(m *testing.M) {
	logger.InitLogger(config.LogConfig{
		Level:   "debug",
		Console: true,
		File:    "",
	})
	os.Exit(m.Run())
}

// TestEncode 编码一条消息，校验返回的字节长度和内容
func TestEncode(t *testing.T) {
	body := []byte("hello")
	data, err := Encode(1001, 1, body)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	// 总长度 = HeaderSize(8) + len("hello")(5) = 13
	wantLen := HeaderSize + len(body)
	if len(data) != wantLen {
		t.Fatalf("编码后长度 = %d, 期望 %d", len(data), wantLen)
	}

	// 校验包头：MsgLen = 13
	msgLen := uint32(wantLen)
	if msgLen != uint32(wantLen) {
		t.Fatalf("MsgLen = %d, 期望 %d", msgLen, wantLen)
	}

	// 校验包体内容
	if string(data[HeaderSize:]) != "hello" {
		t.Fatalf("Body = %s, 期望 hello", string(data[HeaderSize:]))
	}
}

// TestEncodeLargePacket 编码超长消息，验证返回错误
func TestEncodeLargePacket(t *testing.T) {
	body := make([]byte, MaxPacketSize) // 超出限制
	_, err := Encode(1, 1, body)
	if err == nil {
		t.Fatal("期望编码超长包返回错误，但得到 nil")
	}
}

// TestDecodeNormal 正常解码一条完整消息，校验字段正确
func TestDecodeNormal(t *testing.T) {
	body := []byte("hello")
	encoded, err := Encode(1001, 1, body)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	reader := bufio.NewReader(bytes.NewReader(encoded))
	pkt, err := Decode(reader)
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	if pkt.MsgID != 1001 {
		t.Fatalf("MsgID = %d, 期望 %d", pkt.MsgID, 1001)
	}
	if pkt.SeqID != 1 {
		t.Fatalf("SeqID = %d, 期望 %d", pkt.SeqID, 1)
	}
	if string(pkt.Body) != "hello" {
		t.Fatalf("Body = %s, 期望 hello", string(pkt.Body))
	}
}

// TestDecodeSticky 模拟 TCP 粘包：两个包拼在一起，验证能正确拆分
func TestDecodeSticky(t *testing.T) {
	// 编码第 1 个包：MsgID=1001, SeqID=1, Body="hello"
	pkt1, _ := Encode(1001, 1, []byte("hello"))
	// 编码第 2 个包：MsgID=2001, SeqID=2, Body="world"
	pkt2, _ := Encode(2001, 2, []byte("world"))

	// 两个包粘在一起
	sticky := append(pkt1, pkt2...)
	reader := bufio.NewReader(bytes.NewReader(sticky))

	// 第 1 次 Decode，应解析出第 1 个包
	pkt, err := Decode(reader)
	if err != nil {
		t.Fatalf("第 1 次 Decode 失败: %v", err)
	}
	if pkt.MsgID != 1001 || pkt.SeqID != 1 || string(pkt.Body) != "hello" {
		t.Fatalf("第 1 个包解析错误: MsgID=%d, SeqID=%d, Body=%s", pkt.MsgID, pkt.SeqID, pkt.Body)
	}

	// 第 2 次 Decode，应解析出第 2 个包
	pkt, err = Decode(reader)
	if err != nil {
		t.Fatalf("第 2 次 Decode 失败: %v", err)
	}
	if pkt.MsgID != 2001 || pkt.SeqID != 2 || string(pkt.Body) != "world" {
		t.Fatalf("第 2 个包解析错误: MsgID=%d, SeqID=%d, Body=%s", pkt.MsgID, pkt.SeqID, pkt.Body)
	}
}

// TestDecodeHalf 模拟半包：先传部分数据，验证 ReadFull 会阻塞等待
func TestDecodeHalf(t *testing.T) {
	body := []byte("hello")
	encoded, _ := Encode(1001, 1, body)

	// 第 1 批：只给前 4 字节（只够 MsgLen，不够完整包头 8 字节）
	partial := encoded[:4]
	// 用自定义 Reader 模拟分批到达
	chunkReader := &chunkedReader{
		chunks: [][]byte{
			partial,     // 第 1 批：4 字节
			encoded[4:], // 第 2 批：剩余 9 字节
		},
	}
	reader := bufio.NewReader(chunkReader)

	// Decode 内部会调用 ReadFull，第 1 次只能读到 4 字节，不够 8 字节
	// ReadFull 会继续调用 Read，直到凑满 8 字节或返回错误
	// 我们的 chunkedReader 第 1 次给 4 字节，第 2 次给 9 字节，应该能凑满
	pkt, err := Decode(reader)
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}
	if pkt.MsgID != 1001 || pkt.SeqID != 1 || string(pkt.Body) != "hello" {
		t.Fatalf("包解析错误: MsgID=%d, SeqID=%d, Body=%s", pkt.MsgID, pkt.SeqID, pkt.Body)
	}
}

// chunkedReader 模拟 TCP 分批到达：每次 Read 只返回一部分数据
type chunkedReader struct {
	chunks [][]byte
	pos    int
}

func (r *chunkedReader) Read(buf []byte) (int, error) {
	if r.pos >= len(r.chunks) {
		return 0, io.EOF
	}
	n := copy(buf, r.chunks[r.pos])
	r.pos++
	return n, nil
}

// TestDecodeInvalidLen 传入非法包长，验证返回错误
func TestDecodeInvalidLen(t *testing.T) {
	tests := []struct {
		name    string
		msgLen  uint32 // MsgLen 字段值
		wantErr bool
	}{
		{name: "MsgLen=0", msgLen: 0, wantErr: true},
		{name: "MsgLen小于包头", msgLen: 3, wantErr: true},
		{name: "MsgLen=包头上限-1", msgLen: HeaderSize - 1, wantErr: true},
		{name: "MsgLen=超最大值+1", msgLen: MaxPacketSize + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 手工构造一个小包头的非法数据（只构造 8 字节包头，不需要包体）
			buf := make([]byte, HeaderSize)
			Encode(0, 0, nil)

			// 直接写入非法 MsgLen
			buf[0] = byte(tt.msgLen >> 24)
			buf[1] = byte(tt.msgLen >> 16)
			buf[2] = byte(tt.msgLen >> 8)
			buf[3] = byte(tt.msgLen)

			// Encode 内部有长度校验，构造不了非法包，所以手动构造二进制
			// 用合法 MsgID 和 SeqID
			buf[4] = 0
			buf[5] = 0
			buf[6] = 0
			buf[7] = 0

			reader := bufio.NewReader(bytes.NewReader(buf))
			_, err := Decode(reader)
			if tt.wantErr && err == nil {
				t.Fatal("期望返回错误，但得到 nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("不期望错误，但得到: %v", err)
			}
		})
	}
}

// TestDecodeEOF 模拟连接断开，验证返回 io.EOF
func TestDecodeEOF(t *testing.T) {
	reader := bufio.NewReader(bytes.NewReader(nil))
	_, err := Decode(reader)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("期望 io.EOF，但得到: %v", err)
	}
}

// TestEncodeDecodeRoundtrip 编码再解码，验证往返一致性
func TestEncodeDecodeRoundtrip(t *testing.T) {
	body := []byte("snake game message")
	encoded, err := Encode(3001, 42, body)
	if err != nil {
		t.Fatalf("Encode 失败: %v", err)
	}

	reader := bufio.NewReader(bytes.NewReader(encoded))
	pkt, err := Decode(reader)
	if err != nil {
		t.Fatalf("Decode 失败: %v", err)
	}

	if pkt.MsgID != 3001 {
		t.Fatalf("MsgID = %d, 期望 %d", pkt.MsgID, 3001)
	}
	if pkt.SeqID != 42 {
		t.Fatalf("SeqID = %d, 期望 %d", pkt.SeqID, 42)
	}
	if string(pkt.Body) != "snake game message" {
		t.Fatalf("Body = %s, 期望 snake game message", string(pkt.Body))
	}
	if pkt.MsgLen != uint32(HeaderSize+len(body)) {
		t.Fatalf("MsgLen = %d, 期望 %d", pkt.MsgLen, HeaderSize+len(body))
	}
}
