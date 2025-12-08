package cryptoutil

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
)

const (
	// RSAKeySize RSA 密钥大小（2048位）
	RSAKeySize = 2048
)

// GenerateKeyPair 生成 RSA 密钥对
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	// 生成私钥
	privKey, err := rsa.GenerateKey(rand.Reader, RSAKeySize)
	if err != nil {
		return "", "", fmt.Errorf("生成 RSA 密钥对失败: %w", err)
	}

	// 编码私钥为 PEM 格式
	privKeyBytes := x509.MarshalPKCS1PrivateKey(privKey)
	privKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privKeyBytes,
	})

	// 编码公钥为 PEM 格式
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("编码公钥失败: %w", err)
	}
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(privKeyPEM), string(pubKeyPEM), nil
}

// GetPublicKeyFingerprint 计算公钥指纹（SHA256）
func GetPublicKeyFingerprint(publicKey string) (string, error) {
	// 解析 PEM 格式的公钥
	block, _ := pem.Decode([]byte(publicKey))
	if block == nil {
		return "", errors.New("无效的公钥格式")
	}

	// 解析公钥
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("解析公钥失败: %w", err)
	}

	// 将公钥编码为 DER 格式
	pubKeyDER, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("编码公钥失败: %w", err)
	}

	// 计算 SHA256 哈希
	hash := sha256.Sum256(pubKeyDER)
	return fmt.Sprintf("%x", hash), nil
}

// EncryptWithPublicKey 使用公钥加密数据
func EncryptWithPublicKey(data []byte, publicKey string) ([]byte, error) {
	// 解析 PEM 格式的公钥
	block, _ := pem.Decode([]byte(publicKey))
	if block == nil {
		return nil, errors.New("无效的公钥格式")
	}

	// 解析公钥
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析公钥失败: %w", err)
	}

	// 类型断言为 RSA 公钥
	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("不是有效的 RSA 公钥")
	}

	// 使用 OAEP 加密
	encrypted, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		rsaPubKey,
		data,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("加密失败: %w", err)
	}

	return encrypted, nil
}

// DecryptWithPrivateKey 使用私钥解密数据
func DecryptWithPrivateKey(encryptedData []byte, privateKey string) ([]byte, error) {
	// 解析 PEM 格式的私钥
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return nil, errors.New("无效的私钥格式")
	}

	// 解析私钥
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// 使用 OAEP 解密
	decrypted, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		privKey,
		encryptedData,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return decrypted, nil
}

// SignData 使用私钥对数据进行签名
func SignData(data []byte, privateKey string) ([]byte, error) {
	// 解析 PEM 格式的私钥
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return nil, errors.New("无效的私钥格式")
	}

	// 解析私钥
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// 计算数据的哈希
	hash := sha256.Sum256(data)

	// 使用私钥签名
	signature, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("签名失败: %w", err)
	}

	return signature, nil
}

// VerifySignature 验证签名
func VerifySignature(data, signature []byte, publicKey string) (bool, error) {
	// 解析 PEM 格式的公钥
	block, _ := pem.Decode([]byte(publicKey))
	if block == nil {
		return false, errors.New("无效的公钥格式")
	}

	// 解析公钥
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("解析公钥失败: %w", err)
	}

	// 类型断言为 RSA 公钥
	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return false, errors.New("不是有效的 RSA 公钥")
	}

	// 计算数据的哈希
	hash := sha256.Sum256(data)

	// 验证签名
	err = rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, hash[:], signature)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// EncryptWithPublicKeyBase64 使用公钥加密数据并返回 Base64 编码的字符串
func EncryptWithPublicKeyBase64(data []byte, publicKey string) (string, error) {
	encrypted, err := EncryptWithPublicKey(data, publicKey)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptWithPrivateKeyBase64 从 Base64 编码的字符串解密数据
func DecryptWithPrivateKeyBase64(encryptedBase64 string, privateKey string) ([]byte, error) {
	encrypted, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return nil, fmt.Errorf("Base64 解码失败: %w", err)
	}
	return DecryptWithPrivateKey(encrypted, privateKey)
}

