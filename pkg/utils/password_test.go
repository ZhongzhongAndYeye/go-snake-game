package utils

import (
	"strings"
	"testing"
)

// TestHashPassword 测试密码加密功能。
func TestHashPassword(t *testing.T) {
	// 测试正常加密
	hash, err := HashPassword("123456")
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword 返回空字符串")
	}

	// 验证哈希格式：应以 $2a$ 开头（bcrypt 标准前缀）
	if !strings.HasPrefix(hash, "$2a$") {
		t.Errorf("哈希格式错误，期望 $2a$ 开头，实际: %s", hash)
	}

	// 验证相同密码每次生成不同哈希（bcrypt 自带随机盐值）
	hash2, err := HashPassword("123456")
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if hash == hash2 {
		t.Error("相同密码生成了相同的哈希，bcrypt 应每次生成不同哈希")
	}
}

// TestCheckPassword 测试密码校验功能。
func TestCheckPassword(t *testing.T) {
	plainPassword := "myPass123"

	hash, err := HashPassword(plainPassword)
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}

	// 测试正确密码
	if !CheckPassword(plainPassword, hash) {
		t.Error("正确密码校验失败")
	}

	// 测试错误密码
	if CheckPassword("wrongPassword", hash) {
		t.Error("错误密码校验通过，不应通过")
	}

	// 测试空密码
	if CheckPassword("", hash) {
		t.Error("空密码校验通过，不应通过")
	}

	// 测试无效的哈希值
	if CheckPassword(plainPassword, "invalid_hash") {
		t.Error("无效哈希校验通过，不应通过")
	}
}

// TestHashAndCheck 测试 HashPassword 与 CheckPassword 的完整流程。
func TestHashAndCheck(t *testing.T) {
	passwords := []string{
		"123456",
		"abc123",
		"test@123",
		"a",
		"",
		"密码",
	}

	for _, pwd := range passwords {
		hash, err := HashPassword(pwd)
		if err != nil {
			t.Errorf("HashPassword(%q) 失败: %v", pwd, err)
			continue
		}

		if !CheckPassword(pwd, hash) {
			t.Errorf("CheckPassword(%q, hash) 失败，密码自身校验不通过", pwd)
		}
	}
}

// TestDefaultCost 验证默认成本因子为 10。
func TestDefaultCost(t *testing.T) {
	hash, err := HashPassword("test")
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}

	// bcrypt 哈希格式：$2a$<cost>$<salt><hash>
	// 解析成本因子
	parts := strings.Split(hash, "$")
	if len(parts) < 4 {
		t.Fatalf("哈希格式错误: %s", hash)
	}

	if parts[2] != "10" {
		t.Errorf("默认成本因子期望为 10，实际: %s", parts[2])
	}
}
