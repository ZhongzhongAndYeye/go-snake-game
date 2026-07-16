package gateway

import (
	"context"
	"errors"
	"time"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"google.golang.org/protobuf/proto"
)

// RegisterHandler 注册请求处理器
func RegisterHandler(s *Session, packet *network.Packet) {
	logger.Info("收到注册请求", "session_id", s.logID(), "seq_id", packet.SeqID)

	// 反序列化请求体
	req := &msg.RegisterReq{}
	if err := proto.Unmarshal(packet.Body, req); err != nil {
		logger.Warn("注册请求反序列化失败", "session_id", s.logID(), "error", err)
		s.SendError(ErrCodeParamError, "请求参数格式错误")
		return
	}

	// 参数校验
	username := req.GetUsername()
	password := req.GetPassword()
	if err := validateRegisterParams(username, password); err != nil {
		logger.Warn("注册参数校验失败", "session_id", s.logID(), "username", username, "error", err)
		s.SendError(ErrCodeParamError, err.Error())
		return
	}

	// 创建 gRPC 请求上下文（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用登录服 gRPC 注册接口
	resp, err := GlobalLoginClient.Register(ctx, username, password)
	if err != nil {
		logger.Error("调用登录服注册接口失败", "session_id", s.logID(), "username", username, "error", err)
		s.SendError(ErrCodeSystemError, "注册失败，请稍后重试")
		return
	}

	// 封装注册响应
	registerResp := &msg.RegisterResp{
		Code:     resp.Code,
		Msg:      resp.Msg,
		PlayerId: resp.PlayerId,
	}

	// 序列化响应体
	body, err := proto.Marshal(registerResp)
	if err != nil {
		logger.Error("注册响应序列化失败", "session_id", s.logID(), "username", username, "error", err)
		s.SendError(ErrCodeSystemError, "系统错误")
		return
	}

	// 发送响应给客户端
	s.Send(&network.Packet{
		MsgID: network.MsgIDRegisterResp,
		SeqID: packet.SeqID,
		Body:  body,
	})

	logger.Info("注册响应发送成功", "session_id", s.logID(), "username", username, "player_id", resp.PlayerId, "code", resp.Code)
}

// LoginHandler 登录请求处理器
func LoginHandler(s *Session, packet *network.Packet) {
	logger.Info("收到登录请求", "session_id", s.logID(), "seq_id", packet.SeqID)

	// 反序列化请求体
	req := &msg.LoginReq{}
	if err := proto.Unmarshal(packet.Body, req); err != nil {
		logger.Warn("登录请求反序列化失败", "session_id", s.logID(), "error", err)
		s.SendError(ErrCodeParamError, "请求参数格式错误")
		return
	}

	// 参数校验
	username := req.GetUsername()
	password := req.GetPassword()
	if err := validateLoginParams(username, password); err != nil {
		logger.Warn("登录参数校验失败", "session_id", s.logID(), "username", username, "error", err)
		s.SendError(ErrCodeParamError, err.Error())
		return
	}

	// 创建 gRPC 请求上下文（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用登录服 gRPC 登录接口
	resp, err := GlobalLoginClient.Login(ctx, username, password)
	if err != nil {
		logger.Error("调用登录服登录接口失败", "session_id", s.logID(), "username", username, "error", err)
		s.SendError(ErrCodeSystemError, "登录失败，请稍后重试")
		return
	}

	// 登录成功，将 playerID 和登录状态写入 Session
	if resp.Code == 0 && resp.PlayerId > 0 {
		s.SetPlayerID(uint64(resp.PlayerId))
		s.SetLogin(true)
		logger.Info("玩家登录成功，Session 绑定玩家 ID", "session_id", s.logID(), "player_id", resp.PlayerId, "username", username)
	}

	// 封装登录响应
	loginResp := &msg.LoginResp{
		Code:     resp.Code,
		Msg:      resp.Msg,
		Token:    resp.Token,
		PlayerId: int64(resp.PlayerId),
	}

	// 序列化响应体
	body, err := proto.Marshal(loginResp)
	if err != nil {
		logger.Error("登录响应序列化失败", "session_id", s.logID(), "username", username, "error", err)
		s.SendError(ErrCodeSystemError, "系统错误")
		return
	}

	// 发送响应给客户端
	s.Send(&network.Packet{
		MsgID: network.MsgIDLoginResp,
		SeqID: packet.SeqID,
		Body:  body,
	})

	logger.Info("登录响应发送成功", "session_id", s.logID(), "username", username, "player_id", resp.PlayerId, "code", resp.Code)
}

// 校验注册参数
func validateRegisterParams(username, password string) error {
	if username == "" {
		return errors.New("用户名为空")
	}
	if len(username) < 3 {
		return errors.New("用户名长度至少3个字符")
	}
	if len(username) > 32 {
		return errors.New("用户名长度最多32个字符")
	}
	if password == "" {
		return errors.New("密码为空")
	}
	if len(password) < 6 {
		return errors.New("密码长度至少6个字符")
	}
	if len(password) > 32 {
		return errors.New("密码长度最多32个字符")
	}
	return nil
}

// 校验登录参数
func validateLoginParams(username, password string) error {
	if username == "" {
		return errors.New("用户名为空")
	}
	if len(username) > 32 {
		return errors.New("用户名长度超过限制")
	}
	if password == "" {
		return errors.New("密码为空")
	}
	if len(password) > 32 {
		return errors.New("密码长度超过限制")
	}
	return nil
}
