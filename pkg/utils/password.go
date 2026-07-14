package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword 生成密码的 bcrypt 哈希值。
// bcrypt 算法特点：
// 1. 自带随机盐值（salt）：每次生成的哈希值都不同，即使密码相同
// 2. 抗彩虹表攻击：由于盐值随机，彩虹表攻击无效
// 3. 可调节成本因子：成本越高，计算越慢，抗暴力破解能力越强
// 参数 password: 明文密码
// 返回: 哈希字符串和可能的错误
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword 校验明文密码与哈希值是否匹配。
// 参数 plainPassword: 用户输入的明文密码
// 参数 hashPassword: 数据库中存储的哈希值
// 返回: 匹配返回 true，不匹配或哈希格式错误返回 false
func CheckPassword(plainPassword, hashPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashPassword), []byte(plainPassword))
	return err == nil
}

// 【加密流程直观流程例子】
// 1.注册加密：密码 123456 → 随机盐 A → 哈希 A：$2b$10$盐A$哈希A
// 2.数据库存储：$2b$10$盐A$哈希A
// 3.用户登录输入 123456：
// 4.从哈希 A 拆分出盐 A
// 5.用盐 A + 输入密码重新计算，得到 $2b$10$盐A$哈希A
// 6.和数据库字符串完全匹配，校验通过
// 7.用户输入错误密码 654321：
// 8.同样用盐 A 加密，得到新哈希后半段和数据库不一致，校验失败
