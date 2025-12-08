package services

import (
	"goravel/app/cryptoutil"
)

// GenerateKeyPair 生成 RSA 密钥对
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	return cryptoutil.GenerateKeyPair()
}

// GetPublicKeyFingerprint 计算公钥指纹（SHA256）
func GetPublicKeyFingerprint(publicKey string) (string, error) {
	return cryptoutil.GetPublicKeyFingerprint(publicKey)
}

// EncryptWithPublicKey 使用公钥加密数据
func EncryptWithPublicKey(data []byte, publicKey string) ([]byte, error) {
	return cryptoutil.EncryptWithPublicKey(data, publicKey)
}

// DecryptWithPrivateKey 使用私钥解密数据
func DecryptWithPrivateKey(encryptedData []byte, privateKey string) ([]byte, error) {
	return cryptoutil.DecryptWithPrivateKey(encryptedData, privateKey)
}

// SignData 使用私钥对数据进行签名
func SignData(data []byte, privateKey string) ([]byte, error) {
	return cryptoutil.SignData(data, privateKey)
}

// VerifySignature 验证签名
func VerifySignature(data, signature []byte, publicKey string) (bool, error) {
	return cryptoutil.VerifySignature(data, signature, publicKey)
}

// EncryptWithPublicKeyBase64 使用公钥加密数据并返回 Base64 编码的字符串
func EncryptWithPublicKeyBase64(data []byte, publicKey string) (string, error) {
	return cryptoutil.EncryptWithPublicKeyBase64(data, publicKey)
}

// DecryptWithPrivateKeyBase64 从 Base64 编码的字符串解密数据
func DecryptWithPrivateKeyBase64(encryptedBase64 string, privateKey string) ([]byte, error) {
	return cryptoutil.DecryptWithPrivateKeyBase64(encryptedBase64, privateKey)
}
