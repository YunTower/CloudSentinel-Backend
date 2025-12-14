package commands

import (
	"crypto/rand"
	"fmt"
	"math/big"
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

