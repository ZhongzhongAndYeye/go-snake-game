package login

import (
	"context"
	"errors"

	"go-snake-game/pkg/logger"
	pb "go-snake-game/pkg/proto/rpc"
	"go-snake-game/pkg/utils"
)

// 业务错误码，统一封装响应中的 code 字段
const (
	CodeSuccess           = 0 // 成功
	CodeInvalidParam      = 1 // 参数格式错误（用户名/密码不符合规则）
	CodeUsernameExists    = 2 // 用户名已存在
	CodeAccountNotFound   = 3 // 账号不存在
	CodePasswordIncorrect = 4 // 密码错误
	CodeRegisterFailed    = 5 // 注册失败（服务端内部错误）
	CodeLoginFailed       = 6 // 登录失败（服务端内部错误）
	CodeTokenNotFound     = 7 // Token 不存在或已过期
	CodeTokenInvalid      = 8 // Token 格式无效
)

// LoginServerImpl 登录 gRPC 服务端实现。
// 嵌入 UnimplementedLoginServiceServer 保证向前兼容，
// 持有 LoginService 业务实例完成实际逻辑。
type LoginServerImpl struct {
	pb.UnimplementedLoginServiceServer
	svc *LoginService
}

// NewLoginServer 创建登录 gRPC 服务端实例。
func NewLoginServer() *LoginServerImpl {
	return &LoginServerImpl{
		svc: NewLoginService(),
	}
}

// Register 注册账号。
func (s *LoginServerImpl) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	logger.Info("gRPC Register", "username", req.GetUsername())

	playerID, err := s.svc.Register(req.GetUsername(), req.GetPassword())
	if err != nil {
		code, msg := mapRegisterError(err)
		logger.Warn("gRPC Register failed", "username", req.GetUsername(), "code", code, "error", err)
		return &pb.RegisterResponse{Code: code, Msg: msg}, nil
	}

	logger.Info("gRPC Register success", "player_id", playerID)
	return &pb.RegisterResponse{Code: CodeSuccess, Msg: "注册成功", PlayerId: playerID}, nil
}

// Login 账号登录。
func (s *LoginServerImpl) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	logger.Info("gRPC Login", "username", req.GetUsername())

	playerID, nickname, token, err := s.svc.Login(req.GetUsername(), req.GetPassword())
	if err != nil {
		code, msg := mapLoginError(err)
		logger.Warn("gRPC Login failed", "username", req.GetUsername(), "code", code, "error", err)
		return &pb.LoginResponse{Code: code, Msg: msg}, nil
	}

	logger.Info("gRPC Login success", "player_id", playerID)
	return &pb.LoginResponse{
		Code:     CodeSuccess,
		Msg:      "登录成功",
		PlayerId: playerID,
		Nickname: nickname,
		Token:    token,
	}, nil
}

// VerifyToken 校验 Token。
func (s *LoginServerImpl) VerifyToken(ctx context.Context, req *pb.VerifyTokenRequest) (*pb.VerifyTokenResponse, error) {
	playerID, err := s.svc.VerifyToken(req.GetToken())
	if err != nil {
		code, msg := mapVerifyTokenError(err)
		logger.Warn("gRPC VerifyToken failed", "code", code, "error", err)
		return &pb.VerifyTokenResponse{Code: code, Msg: msg}, nil
	}

	logger.Info("gRPC VerifyToken success", "player_id", playerID)
	return &pb.VerifyTokenResponse{Code: CodeSuccess, Msg: "Token 有效", PlayerId: playerID}, nil
}

// mapRegisterError 将注册业务错误映射为错误码和提示信息。
func mapRegisterError(err error) (int32, string) {
	switch {
	case errors.Is(err, ErrUsernameEmpty),
		errors.Is(err, ErrUsernameTooShort),
		errors.Is(err, ErrUsernameTooLong),
		errors.Is(err, ErrPasswordEmpty),
		errors.Is(err, ErrPasswordTooShort),
		errors.Is(err, ErrPasswordTooLong):
		return CodeInvalidParam, "参数格式错误"
	case errors.Is(err, ErrUsernameExists):
		return CodeUsernameExists, "用户名已存在"
	default:
		return CodeRegisterFailed, "注册失败，请稍后重试"
	}
}

// mapLoginError 将登录业务错误映射为错误码和提示信息。
func mapLoginError(err error) (int32, string) {
	switch {
	case errors.Is(err, ErrUsernameEmpty),
		errors.Is(err, ErrUsernameTooShort),
		errors.Is(err, ErrUsernameTooLong),
		errors.Is(err, ErrPasswordEmpty),
		errors.Is(err, ErrPasswordTooShort),
		errors.Is(err, ErrPasswordTooLong):
		return CodeInvalidParam, "参数格式错误"
	case errors.Is(err, ErrAccountNotFound):
		return CodeAccountNotFound, "账号不存在"
	case errors.Is(err, ErrPasswordIncorrect):
		return CodePasswordIncorrect, "密码错误"
	default:
		return CodeLoginFailed, "登录失败，请稍后重试"
	}
}

// mapVerifyTokenError 将 Token 校验错误映射为错误码和提示信息。
func mapVerifyTokenError(err error) (int32, string) {
	switch {
	case errors.Is(err, utils.ErrTokenNotFound):
		return CodeTokenNotFound, "Token 无效或已过期"
	case errors.Is(err, utils.ErrTokenInvalid):
		return CodeTokenInvalid, "Token 格式无效"
	default:
		return CodeTokenInvalid, "Token 校验失败"
	}
}
