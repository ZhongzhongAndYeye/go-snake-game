package utils

// 导入必要的包
import (
	"context"
	"crypto/rand" // 生成安全的随机数
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"go-snake-game/pkg/db" // 引入全局 Redis 客户端

	"github.com/redis/go-redis/v9" // go-redis 客户端库
)

// 预定义的错误变量，供调用方判断错误类型
var (
	ErrTokenNotFound = errors.New("token 不存在或已过期") // Token 不存在或已过期
	ErrTokenInvalid  = errors.New("token 格式无效")    // Token 格式无效（存储的 playerID 不是数字）
)

// GenerateToken 生成登录 Token。
// Redis 存储 Token 的优势：
// 1. 支持主动失效：调用 RemoveToken 可立即删除 Token，适用于退出登录、踢人等场景
// 2. 过期自动清理：Redis 会自动删除过期的 Token，无需手动维护
// 3. 优于 JWT：JWT 一旦签发无法主动作废，只能依赖过期时间，而 Redis Token 可随时失效
// 参数 playerID: 玩家 ID（uint64 类型）
// 返回: Token 字符串和可能的错误
func GenerateToken(playerID uint64) (string, error) {
	// 生成 32 位随机字符串作为 Token 值
	// 32 位十六进制字符串 = 16 字节二进制数据，足够安全（2^128 种可能）
	token, err := generateRandomString(32)
	if err != nil {
		// 如果生成随机字符串失败，返回空字符串和错误
		return "", err
	}

	// 创建一个带 5 秒超时的上下文
	// 防止 Redis 操作长时间阻塞，超时后自动取消
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 将 Token 存入 Redis：【key：token; value: playerID】
	err = db.GlobalRedis.Set(ctx, token, strconv.FormatUint(playerID, 10), 7*24*time.Hour).Err()
	if err != nil {
		// 如果 Redis 设置失败（如连接断开），返回空字符串和错误
		return "", err
	}

	// 成功，返回生成的 Token
	return token, nil
}

// VerifyToken 校验传来的token是否合法或在redis中存在
func VerifyToken(token string) (uint64, error) {
	// 创建一个带 5 秒超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 从 Redis 中获取 Token 对应的 playerID
	val, err := db.GlobalRedis.Get(ctx, token).Result()
	if err != nil {
		// 如果出现错误
		if err == redis.Nil {
			// redis.Nil 表示 key 不存在，说明 Token 无效或已过期
			return 0, ErrTokenNotFound
		}
		// 其他错误（如 Redis 连接问题），直接返回错误
		return 0, err
	}

	// 将 Redis 返回的字符串转换为 uint64 类型的 playerID
	// base=10 表示十进制，bitSize=64 表示转换为 64 位无符号整数
	playerID, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		// 转换失败，说明 Redis 中存储的值不是有效的数字格式
		return 0, ErrTokenInvalid
	}

	// 校验成功，返回玩家 ID
	return playerID, nil
}

// RemoveToken 主动删除 Token，适用于退出登录、踢人等场景。
func RemoveToken(token string) error {
	// 创建一个带 5 秒超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 从 Redis 中删除 Token 对应的 key
	// Del 命令会返回被删除的 key 数量，这里我们只关心是否出错
	return db.GlobalRedis.Del(ctx, token).Err()
}

// generateRandomString 生成指定长度的随机字符串。
// 使用 crypto/rand 生成安全的随机数，比 math/rand 更安全（密码学安全）。
// 参数 length: 生成的字符串长度（十六进制字符串）
// 返回: 随机字符串和可能的错误
func generateRandomString(length int) (string, error) {
	// 创建一个长度为 length/2 的字节数组，因为俩个字符（单个string）占一个字节(byte)
	bytes := make([]byte, length/2)

	// 使用 crypto/rand.Read 填充字节数组：从系统熵池读取 16 个随机字节填充进去
	// crypto/rand 是密码学安全的随机数生成器，适合生成敏感数据（如 Token）
	_, err := rand.Read(bytes)
	if err != nil {
		// 如果读取失败（如系统熵池耗尽），返回错误
		return "", err
	}

	// 将二进制字节数组转换为十六进制字符串
	// 例如: [0x1a, 0x3b] → "1a3b"
	return hex.EncodeToString(bytes), nil
}
