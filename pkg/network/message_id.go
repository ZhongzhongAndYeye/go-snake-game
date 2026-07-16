package network

// 消息 ID 常量定义（uint16）
// 命名规则：MsgID + 业务模块 + 方向（Req/Resp）
// 范围划分：
//   - 1000-1999：基础通信（心跳、登录、错误）
//   - 2000-2999：房间匹配
const (
	// ---- 心跳相关 ----
	MsgIDHeartbeatReq  uint16 = 1001 // 客户端 → 服务端：心跳请求
	MsgIDHeartbeatResp uint16 = 1002 // 服务端 → 客户端：心跳响应

	// ---- 登录相关 ----
	MsgIDLoginReq  uint16 = 1003 // 客户端 → 登录服：登录请求
	MsgIDLoginResp uint16 = 1004 // 登录服 → 客户端：登录响应

	// ---- 通用错误 ----
	MsgIDErrorResp uint16 = 1005 // 服务端 → 客户端：通用错误响应

	// ---- 注册相关 ----
	MsgIDRegisterReq  uint16 = 1006 // 客户端 → 登录服：注册请求
	MsgIDRegisterResp uint16 = 1007 // 登录服 → 客户端：注册响应

	// ---- 房间匹配相关 ----
	MsgIDMatchStartReq   uint16 = 2001 // 客户端 → 游戏服：发起匹配请求
	MsgIDMatchStartResp  uint16 = 2002 // 游戏服 → 客户端：发起匹配响应
	MsgIDMatchCancelReq  uint16 = 2003 // 客户端 → 游戏服：取消匹配请求
	MsgIDMatchCancelResp uint16 = 2004 // 游戏服 → 客户端：取消匹配响应
	MsgIDRoomInfoNotify  uint16 = 2005 // 游戏服 → 客户端：房间信息推送
)
