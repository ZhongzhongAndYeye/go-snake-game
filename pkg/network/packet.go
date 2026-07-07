package network

// 包头大小：4字节 MsgLen + 2字节 MsgID + 2字节 SeqID = 8字节
const HeaderSize = 8

// 最大包长限制，防止恶意大包攻击
const MaxPacketSize = 65535

// Packet 游戏服务器二进制消息包结构
type Packet struct {
	// MsgLen 包总长度（包含包头+包体），uint32 占 4 字节
	MsgLen uint32

	// MsgID 消息类型 ID，用于路由到对应的消息处理器，uint16 占 2 字节
	MsgID uint16

	// SeqID 消息序列号，用于请求-响应匹配和消息排序，uint16 占 2 字节
	// 一般客户端发来，服务端回复时会使用相同的 SeqID
	SeqID uint16

	// Body 消息体，实际业务数据
	Body []byte
}
