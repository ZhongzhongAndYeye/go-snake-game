package network

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"go-snake-game/pkg/logger"
)

// Encode 将消息编码为二进制字节流。
// 包头结构（大端序）：
//   - 0-3 字节：MsgLen（uint32，包总长度 = HeaderSize + len(body)）
//   - 4-5 字节：MsgID（uint16，消息类型 ID）
//   - 6-7 字节：SeqID（uint16，消息序列号）
//
// 包体紧跟在包头之后。
func Encode(msgID uint16, seqID uint16, body []byte) ([]byte, error) {
	// 计算包总长度：包头大小 + 包体大小
	bodyLen := len(body)
	msgLen := uint32(HeaderSize + bodyLen)

	// 校验包总长度，超过最大值返回错误，防止恶意大包攻击
	if int(msgLen) > MaxPacketSize {
		logger.Error("编码失败：包体过大",
			"msg_id", msgID,
			"seq_id", seqID,
			"body_len", bodyLen,
			"max_size", MaxPacketSize,
		)
		return nil, fmt.Errorf("packet too large: %d bytes exceeds max %d bytes", msgLen, MaxPacketSize)
	}

	// 分配足够大的字节缓冲区，存放完整的包头 + 包体
	buf := make([]byte, msgLen)

	// 写入 MsgLen（4字节，大端序）(0 1 2 3)
	binary.BigEndian.PutUint32(buf[0:4], msgLen)

	// 写入 MsgID（2字节，大端序）(4 5)
	binary.BigEndian.PutUint16(buf[4:6], msgID)

	// 写入 SeqID（2字节，大端序）(6 7)
	binary.BigEndian.PutUint16(buf[6:8], seqID)

	// 写入 Body（包体数据）(8-)
	copy(buf[HeaderSize:], body)

	// 调试日志：打印编码结果
	logger.Debug("消息编码完成",
		"msg_id", msgID,
		"seq_id", seqID,
		"total_len", msgLen,
		"body_len", bodyLen,
	)

	return buf, nil
}

// Decode 从 bufio.Reader 中解码一个完整的 Packet。
// ！！！核心原理： 使用 ReadFull 阻塞等待，直到读满包头和包体，天然解决 TCP 半包问题。（没收全，一直等）
// ReadFull 读走数据后，会自动从缓冲区移除已读的部分。
// 若是解码失败了，比如msgLen不合法，直接关闭连接，这样就不用担心tcp连接缓冲区失真的问题
// 当然，解码失败关闭连接不是这里需要处理的事情，这里只需要在失败时返回错误就行了（失败，关连接）
// 同时也兼容粘包场景——每次只解码一个包，上层读走已解码部分即可。（缓冲区多包，一次只取一个包）
//
// 工作流程：
//
//	第一步：ReadFull 读满 8 字节包头 → 解析出 MsgLen
//	第二步：校验 MsgLen 合法性
//	第三步：ReadFull 读满剩余包体 → 组装 Packet 返回
func Decode(reader *bufio.Reader) (*Packet, error) {
	// ---- 第一步：读满 8 字节包头 ----
	// io.ReadFull 会阻塞等待，直到正好读到 HeaderSize 字节
	// 如果连接关闭或数据不足，返回 io.EOF 或 io.ErrUnexpectedEOF
	headerBuf := make([]byte, HeaderSize) // HeaderSize = 8

	// 错误处理
	if _, err := io.ReadFull(reader, headerBuf); err != nil {
		logger.Error("解码失败：读取包头出错", "error", err.Error())
		// 将 io.EOF 原样返回，方便上层判断连接是否已关闭
		// 其他错误（如连接重置）统一包装返回
		if errors.Is(err, io.EOF) {
			return nil, err
		}
		return nil, fmt.Errorf("read header: %w", err)
	}

	// 解析包头：MsgLen（4字节）、MsgID（2字节）、SeqID（2字节），均为大端序
	msgLen := binary.BigEndian.Uint32(headerBuf[0:4])
	msgID := binary.BigEndian.Uint16(headerBuf[4:6])
	seqID := binary.BigEndian.Uint16(headerBuf[6:8])

	// ---- 第二步：校验 MsgLen 合法性 ----
	// 包总长度不能小于包头大小，防止非法的极小值数据
	if msgLen < HeaderSize {
		logger.Error("解码失败：包长非法",
			"msg_len", msgLen,
			"header_size", HeaderSize,
		)
		return nil, fmt.Errorf("invalid msgLen: %d, must be at least %d", msgLen, HeaderSize)
	}
	// 包总长度不能超过最大值，防止恶意大包攻击
	if int(msgLen) > MaxPacketSize {
		logger.Error("解码失败：包体过大",
			"msg_len", msgLen,
			"max_size", MaxPacketSize,
		)
		return nil, fmt.Errorf("packet too large: %d bytes exceeds max %d bytes", msgLen, MaxPacketSize)
	}

	// ---- 第三步：计算包体长度，读满包体 ----
	bodyLen := int(msgLen - HeaderSize)
	body := make([]byte, bodyLen)
	if bodyLen > 0 {
		// 用 ReadFull 读取完整包体，阻塞等待直到数据到齐
		// 如果连接在此期间关闭，返回 io.EOF / io.ErrUnexpectedEOF
		if _, err := io.ReadFull(reader, body); err != nil {
			logger.Error("解码失败：读取包体出错",
				"error", err.Error(),
				"body_len", bodyLen,
			)
			if errors.Is(err, io.EOF) {
				return nil, err
			}
			return nil, fmt.Errorf("read body: %w", err)
		}
	}

	// 调试日志：打印解码结果
	logger.Debug("消息解码完成",
		"msg_id", msgID,
		"seq_id", seqID,
		"total_len", msgLen,
		"body_len", bodyLen,
	)

	return &Packet{
		MsgLen: msgLen,
		MsgID:  msgID,
		SeqID:  seqID,
		Body:   body,
	}, nil
}
