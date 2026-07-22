package service

import (
	"errors"

	"go-snake-game/internal/dao"
	"go-snake-game/internal/model"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/utils"
)

// 业务错误常量，用于区分不同的错误类型，方便调用方处理
var (
	ErrUsernameEmpty     = errors.New("用户名为空")
	ErrUsernameTooShort  = errors.New("用户名长度至少3个字符")
	ErrUsernameTooLong   = errors.New("用户名长度最多32个字符")
	ErrPasswordEmpty     = errors.New("密码为空")
	ErrPasswordTooShort  = errors.New("密码长度至少6个字符")
	ErrPasswordTooLong   = errors.New("密码长度最多32个字符")
	ErrUsernameExists    = errors.New("用户名已存在")
	ErrAccountNotFound   = errors.New("账号不存在")
	ErrPasswordIncorrect = errors.New("密码错误")
	ErrRegisterFailed    = errors.New("注册失败")
	ErrLoginFailed       = errors.New("登录失败")
)

// LoginService 登录服务，处理用户注册、登录、Token 校验等业务逻辑。
// 无状态服务，可并发安全使用。
type LoginService struct{}

// NewLoginService 创建登录服务实例。
func NewLoginService() *LoginService {
	return &LoginService{}
}

// Register 用户注册。
// 参数 username: 用户名（3-32 位）
// 参数 password: 密码（6-32 位）
// 返回: 玩家 ID 和可能的错误
func (s *LoginService) Register(username, password string) (uint64, error) {
	logger.Info("register request", "username", username)

	// 校验用户名格式
	if err := validateUsername(username); err != nil {
		logger.Warn("invalid username", "username", username, "error", err)
		return 0, err
	}

	// 校验密码格式
	if err := validatePassword(password); err != nil {
		logger.Warn("invalid password", "username", username, "error", err)
		return 0, err
	}

	// 查询用户名是否已存在
	player, err := dao.GetPlayerByUsername(username)
	if err == nil {
		// 查询成功说明用户名已存在
		logger.Warn("username already exists", "username", username, "player_id", player.ID)
		return 0, ErrUsernameExists
	}
	if !errors.Is(err, dao.ErrPlayerNotFound) {
		// 数据库异常
		logger.Error("failed to check username", "username", username, "error", err)
		return 0, err
	}

	// 密码 bcrypt 加密
	hash, err := utils.HashPassword(password)
	if err != nil {
		logger.Error("failed to hash password", "username", username, "error", err)
		return 0, ErrRegisterFailed
	}

	// 创建玩家记录，昵称默认等于用户名
	newPlayer := &model.Player{
		Username:     username,
		PasswordHash: hash,
		Nickname:     username,
	}

	// 插入数据库
	err = dao.CreatePlayer(newPlayer)
	if err != nil {
		logger.Error("failed to create player", "username", username, "error", err)
		return 0, ErrRegisterFailed
	}

	logger.Info("register success", "username", username, "player_id", newPlayer.ID)
	return newPlayer.ID, nil
}

// Login 用户登录。
// 参数 username: 用户名
// 参数 password: 密码
// 返回: 玩家 ID、昵称、Token 和可能的错误
func (s *LoginService) Login(username, password string) (uint64, string, string, error) {
	logger.Info("login request", "username", username)

	// 校验用户名格式
	if err := validateUsername(username); err != nil {
		logger.Warn("invalid username", "username", username, "error", err)
		return 0, "", "", ErrAccountNotFound
	}

	// 查询玩家信息
	player, err := dao.GetPlayerByUsername(username)
	if err != nil {
		if errors.Is(err, dao.ErrPlayerNotFound) {
			logger.Warn("account not found", "username", username)
			return 0, "", "", ErrAccountNotFound
		}
		logger.Error("failed to get player", "username", username, "error", err)
		return 0, "", "", ErrLoginFailed
	}

	// 校验密码
	if !utils.CheckPassword(password, player.PasswordHash) {
		logger.Warn("password incorrect", "username", username, "player_id", player.ID)
		return 0, "", "", ErrPasswordIncorrect
	}

	// 生成登录 Token（存储到 Redis，有效期 7 天）
	token, err := utils.GenerateToken(player.ID)
	if err != nil {
		logger.Error("failed to generate token", "username", username, "player_id", player.ID, "error", err)
		return 0, "", "", ErrLoginFailed
	}

	logger.Info("login success", "username", username, "player_id", player.ID)
	return player.ID, player.Nickname, token, nil
}

// VerifyToken 校验 Token 是否有效。
// 参数 token: 待校验的 Token
// 返回: 玩家 ID 和可能的错误
func (s *LoginService) VerifyToken(token string) (uint64, error) {
	playerID, err := utils.VerifyToken(token)
	if err != nil {
		if errors.Is(err, utils.ErrTokenNotFound) {
			logger.Warn("token not found", "token", token)
			return 0, err
		}
		logger.Error("failed to verify token", "token", token, "error", err)
		return 0, err
	}

	return playerID, nil
}

// validateUsername 校验用户名格式。
// 要求：非空，长度 3-32 位
func validateUsername(username string) error {
	if username == "" {
		return ErrUsernameEmpty
	}
	if len(username) < 3 {
		return ErrUsernameTooShort
	}
	if len(username) > 32 {
		return ErrUsernameTooLong
	}
	return nil
}

// validatePassword 校验密码格式。
// 要求：非空，长度 6-32 位
func validatePassword(password string) error {
	if password == "" {
		return ErrPasswordEmpty
	}
	if len(password) < 6 {
		return ErrPasswordTooShort
	}
	if len(password) > 32 {
		return ErrPasswordTooLong
	}
	return nil
}
