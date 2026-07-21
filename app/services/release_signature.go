package services

import (
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

const releaseSigningKeyFingerprint = "2AB82536577C0A9F52347022CF62CB23EB21D3D0"

//go:embed release_signing_key.asc
var releaseSigningPublicKey string

// VerifyReleaseSignature checks an armored detached signature against the
// embedded YunTower release key. It has no dependency on a system gpg binary.
func VerifyReleaseSignature(signedPath, signaturePath string) error {
	keyring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(releaseSigningPublicKey))
	if err != nil {
		return fmt.Errorf("读取内置发布公钥失败: %w", err)
	}

	signedFile, err := os.Open(signedPath)
	if err != nil {
		return fmt.Errorf("打开待验签文件失败: %w", err)
	}
	defer signedFile.Close()

	signatureFile, err := os.Open(signaturePath)
	if err != nil {
		return fmt.Errorf("打开签名文件失败: %w", err)
	}
	defer signatureFile.Close()

	signer, err := openpgp.CheckArmoredDetachedSignature(keyring, signedFile, signatureFile, nil)
	if err != nil {
		return fmt.Errorf("发布签名校验失败: %w", err)
	}
	if signer == nil || signer.PrimaryKey == nil {
		return fmt.Errorf("发布签名缺少签名者公钥")
	}

	actualFingerprint := strings.ToUpper(hex.EncodeToString(signer.PrimaryKey.Fingerprint[:]))
	if actualFingerprint != releaseSigningKeyFingerprint {
		return fmt.Errorf("发布签名者不受信任: %s", actualFingerprint)
	}

	return nil
}
