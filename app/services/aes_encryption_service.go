package services

import (
	"goravel/app/cryptoutil"
)

const (
	// AESKeySize AES 密钥大小（256位 = 32字节）
	AESKeySize = cryptoutil.AESKeySize
	// GCMNonceSize GCM nonce 大小（12字节）
	GCMNonceSize = cryptoutil.GCMNonceSize
)

// GenerateSessionKey 生成 AES-256 会话密钥
func GenerateSessionKey() ([]byte, error) {
	return cryptoutil.GenerateSessionKey()
}

// EncryptMessage 使用 AES-GCM 加密消息
func EncryptMessage(message []byte, key []byte) ([]byte, error) {
	return cryptoutil.EncryptMessage(message, key)
}

// DecryptMessage 使用 AES-GCM 解密消息
func DecryptMessage(encryptedMessage []byte, key []byte) ([]byte, error) {
	return cryptoutil.DecryptMessage(encryptedMessage, key)
}
