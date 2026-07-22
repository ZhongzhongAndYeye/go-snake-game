package login

import (
	"context"
	"testing"
	"time"

	"go-snake-game/internal/login/service"
	"go-snake-game/internal/model"
	"go-snake-game/pkg/config"
	"go-snake-game/pkg/db"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/utils"
)

// setup 初始化测试环境：日志、MySQL、Redis。
func setup(t *testing.T) {
	t.Helper()

	// 初始化日志
	_ = logger.InitLogger(config.LogConfig{
		Level:   "error",
		Console: false,
	})

	// 加载配置文件
	err := config.InitConfig("../../configs/dev.yaml")
	if err != nil {
		t.Fatalf("加载配置文件失败: %v", err)
	}

	// 初始化 MySQL
	mysqlCfg := config.GlobalCfg.Mysql
	db.InitMySQL(&db.MySQLConfig{
		DSN:            mysqlCfg.DSN,
		MaxOpen:        mysqlCfg.MaxOpen,
		MaxIdle:        mysqlCfg.MaxIdle,
		MaxLifeMinutes: mysqlCfg.MaxLifeMinutes,
	})

	// 自动迁移表结构
	err = db.GlobalDB.AutoMigrate(&model.Player{})
	if err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}

	// 初始化 Redis
	redisCfg := config.GlobalCfg.Redis
	db.InitRedis(&db.RedisConfig{
		Addr:         redisCfg.Addr,
		DB:           redisCfg.DB,
		Password:     redisCfg.Password,
		PoolSize:     redisCfg.PoolSize,
		MinIdleConns: redisCfg.MinIdleConns,
		MaxRetries:   redisCfg.MaxRetries,
		DialTimeout:  redisCfg.DialTimeout,
		ReadTimeout:  redisCfg.ReadTimeout,
		WriteTimeout: redisCfg.WriteTimeout,
		PoolTimeout:  redisCfg.PoolTimeout,
	})
}

// cleanupPlayer 删除测试创建的玩家数据。
func cleanupPlayer(t *testing.T, username string) {
	t.Helper()
	db.GlobalDB.Where("username = ?", username).Delete(&model.Player{})
}

// TestRegister 测试注册成功场景。
func TestRegister(t *testing.T) {
	setup(t)

	username := "test_register_user"
	password := "testPass123"
	defer cleanupPlayer(t, username)

	svc := service.NewLoginService()
	playerID, err := svc.Register(username, password)
	if err != nil {
		t.Fatalf("Register 失败: %v", err)
	}
	if playerID == 0 {
		t.Fatal("注册成功但 playerID 为 0")
	}
	t.Logf("注册成功，playerID: %d", playerID)
}

// TestRegisterDuplicate 测试重复注册应返回 ErrUsernameExists。
func TestRegisterDuplicate(t *testing.T) {
	setup(t)

	username := "test_register_dup"
	password := "testPass123"
	defer cleanupPlayer(t, username)

	svc := service.NewLoginService()

	// 第一次注册应成功
	_, err := svc.Register(username, password)
	if err != nil {
		t.Fatalf("第一次注册失败: %v", err)
	}

	// 第二次注册相同用户名应失败
	_, err = svc.Register(username, "otherPass456")
	if err != service.ErrUsernameExists {
		t.Errorf("重复注册应返回 ErrUsernameExists，实际: %v", err)
	}
}

// TestRegisterInvalidParams 测试注册参数校验。
func TestRegisterInvalidParams(t *testing.T) {
	setup(t)
	svc := service.NewLoginService()

	tests := []struct {
		name     string
		username string
		password string
		wantErr  error
	}{
		{"空用户名", "", "123456", service.ErrUsernameEmpty},
		{"用户名太短", "ab", "123456", service.ErrUsernameTooShort},
		{"用户名太长", string(make([]byte, 33)), "123456", service.ErrUsernameTooLong},
		{"空密码", "testuser", "", service.ErrPasswordEmpty},
		{"密码太短", "testuser", "12345", service.ErrPasswordTooShort},
		{"密码太长", "testuser", string(make([]byte, 33)), service.ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(tt.username, tt.password)
			if err != tt.wantErr {
				t.Errorf("期望错误 %v，实际 %v", tt.wantErr, err)
			}
		})
	}
}

// TestLogin 测试登录成功场景。
func TestLogin(t *testing.T) {
	setup(t)

	username := "test_login_ok"
	password := "loginPass123"
	defer cleanupPlayer(t, username)

	svc := service.NewLoginService()

	// 先注册
	_, err := svc.Register(username, password)
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 登录
	playerID, nickname, token, err := svc.Login(username, password)
	if err != nil {
		t.Fatalf("Login 失败: %v", err)
	}
	if playerID == 0 {
		t.Fatal("登录成功但 playerID 为 0")
	}
	if nickname == "" {
		t.Fatal("登录成功但 nickname 为空")
	}
	if token == "" {
		t.Fatal("登录成功但 token 为空")
	}
	t.Logf("登录成功，playerID: %d, nickname: %s, token: %s", playerID, nickname, token)
}

// TestLoginPasswordIncorrect 测试密码错误登录。
func TestLoginPasswordIncorrect(t *testing.T) {
	setup(t)

	username := "test_login_wrong_pwd"
	password := "correctPass123"
	wrongPassword := "wrongPass456"
	defer cleanupPlayer(t, username)

	svc := service.NewLoginService()

	// 先注册
	_, err := svc.Register(username, password)
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 使用错误密码登录
	_, _, _, err = svc.Login(username, wrongPassword)
	if err != service.ErrPasswordIncorrect {
		t.Errorf("错误密码登录应返回 ErrPasswordIncorrect，实际: %v", err)
	}
}

// TestLoginAccountNotFound 测试不存在的账号登录。
func TestLoginAccountNotFound(t *testing.T) {
	setup(t)
	svc := service.NewLoginService()

	_, _, _, err := svc.Login("not_exist_user_xxx", "anyPass123")
	if err != service.ErrAccountNotFound {
		t.Errorf("不存在的账号登录应返回 ErrAccountNotFound，实际: %v", err)
	}
}

// TestVerifyToken 测试 Token 校验。
func TestVerifyToken(t *testing.T) {
	setup(t)

	username := "test_verify_token"
	password := "verifyPass123"
	defer cleanupPlayer(t, username)

	svc := service.NewLoginService()

	// 先注册
	_, err := svc.Register(username, password)
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 登录获取 Token
	playerID, _, token, err := svc.Login(username, password)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	// 校验 Token
	verifiedID, err := svc.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken 失败: %v", err)
	}
	if verifiedID != playerID {
		t.Errorf("Token 校验返回的 ID 不匹配，期望 %d，实际 %d", playerID, verifiedID)
	}
	t.Logf("Token 校验成功，playerID: %d", verifiedID)
}

// TestVerifyTokenInvalid 测试无效 Token 校验。
func TestVerifyTokenInvalid(t *testing.T) {
	setup(t)
	svc := service.NewLoginService()

	// 使用无效 Token
	_, err := svc.VerifyToken("invalid_token_xxx")
	if err != utils.ErrTokenNotFound {
		t.Errorf("无效 Token 应返回 ErrTokenNotFound，实际: %v", err)
	}
}

// TestLoginAndVerifyFlow 测试完整的注册→登录→Token 校验流程。
func TestLoginAndVerifyFlow(t *testing.T) {
	setup(t)

	username := "test_full_flow"
	password := "flowPass123"
	defer cleanupPlayer(t, username)

	svc := service.NewLoginService()

	// 1. 注册
	playerID, err := svc.Register(username, password)
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	t.Logf("1. 注册成功，playerID: %d", playerID)

	// 2. 登录
	loginID, nickname, token, err := svc.Login(username, password)
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if loginID != playerID {
		t.Errorf("登录返回的 ID 与注册不一致，注册: %d，登录: %d", playerID, loginID)
	}
	t.Logf("2. 登录成功，nickname: %s", nickname)

	// 3. 校验 Token
	verifiedID, err := svc.VerifyToken(token)
	if err != nil {
		t.Fatalf("Token 校验失败: %v", err)
	}
	if verifiedID != playerID {
		t.Errorf("Token 校验 ID 不匹配，期望 %d，实际 %d", playerID, verifiedID)
	}
	t.Log("3. Token 校验成功")

	// 4. 使用错误密码登录
	_, _, _, err = svc.Login(username, "wrongPass")
	if err != service.ErrPasswordIncorrect {
		t.Errorf("错误密码应返回 ErrPasswordIncorrect，实际: %v", err)
	}
	t.Log("4. 错误密码校验通过")

	// 5. 清理 Token（模拟退出登录）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db.GlobalRedis.Del(ctx, token)

	// 6. Token 删除后校验应失败
	_, err = svc.VerifyToken(token)
	if err != utils.ErrTokenNotFound {
		t.Errorf("Token 删除后应返回 ErrTokenNotFound，实际: %v", err)
	}
	t.Log("5-6. Token 删除后校验失败，符合预期")
}
