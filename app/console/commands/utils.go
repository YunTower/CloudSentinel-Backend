package commands

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/goravel/framework/facades"
)

// GenerateRandomString 生成随机字符串（公共函数）
// length: 字符串长度
// charset: "alphanumeric" (字母+数字) 或 "alphanumeric_special" (字母+数字+特殊字符)
func GenerateRandomString(length int, charset string) (string, error) {
	var chars string
	switch charset {
	case "alphanumeric":
		// 字母和数字（排除容易混淆的字符：0, O, I, l）
		chars = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	case "alphanumeric_special":
		// 字母、数字和特殊字符（排除容易混淆的字符）
		chars = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*"
	default:
		chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	}

	result := make([]byte, length)
	charsLen := big.NewInt(int64(len(chars)))

	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, charsLen)
		if err != nil {
			return "", fmt.Errorf("生成随机数失败: %w", err)
		}
		result[i] = chars[randomIndex.Int64()]
	}

	return string(result), nil
}

// AdminCredentials 管理员凭证结构
type AdminCredentials struct {
	Username     string
	Password     string
	PasswordHash string
}

// generateAdminCredentials 生成管理员凭证（统一的管理员账号生成逻辑）
func generateAdminCredentials() (*AdminCredentials, error) {
	// 生成10位随机用户名（字母和数字）
	username, err := GenerateRandomString(10, "alphanumeric")
	if err != nil {
		return nil, fmt.Errorf("生成用户名失败: %w", err)
	}

	// 生成20位随机密码（字母、数字和特殊字符）
	password, err := GenerateRandomString(20, "alphanumeric_special")
	if err != nil {
		return nil, fmt.Errorf("生成密码失败: %w", err)
	}

	// 生成密码哈希
	passwordHash, err := facades.Hash().Make(password)
	if err != nil {
		return nil, fmt.Errorf("生成密码哈希失败: %w", err)
	}

	return &AdminCredentials{
		Username:     username,
		Password:     password,
		PasswordHash: passwordHash,
	}, nil
}

