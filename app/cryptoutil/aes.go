package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	// AESKeySize AES 密钥大小（256位 = 32字节）
	AESKeySize = 32
	// GCMNonceSize GCM nonce 大小（12字节）
	GCMNonceSize = 12
)

// GenerateSessionKey 生成 AES-256 会话密钥
func GenerateSessionKey() ([]byte, error) {
	key := make([]byte, AESKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("生成会话密钥失败: %w", err)
	}
	return key, nil
}

// EncryptMessage 使用 AES-GCM 加密消息
func EncryptMessage(message []byte, key []byte) ([]byte, error) {
	// 验证密钥长度
	if len(key) != AESKeySize {
		return nil, fmt.Errorf("密钥长度必须为 %d 字节", AESKeySize)
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 生成 nonce
	nonce := make([]byte, GCMNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密消息
	ciphertext := gcm.Seal(nonce, nonce, message, nil)

	return ciphertext, nil
}

// DecryptMessage 使用 AES-GCM 解密消息
func DecryptMessage(encryptedMessage []byte, key []byte) ([]byte, error) {
	// 验证密钥长度
	if len(key) != AESKeySize {
		return nil, fmt.Errorf("密钥长度必须为 %d 字节", AESKeySize)
	}

	// 验证加密消息长度
	if len(encryptedMessage) < GCMNonceSize {
		return nil, errors.New("加密消息长度不足")
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 提取 nonce 和密文
	nonce := encryptedMessage[:GCMNonceSize]
	ciphertext := encryptedMessage[GCMNonceSize:]

	// 解密消息
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}





