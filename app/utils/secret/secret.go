package secret

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"goravel/app/cryptoutil"
	"github.com/goravel/framework/facades"
)

func appKeyBytes() []byte {
	k := facades.Config().GetString("app.key")
	if k == "" {
		return make([]byte, cryptoutil.AESKeySize)
	}
	b := []byte(k)
	if len(b) >= cryptoutil.AESKeySize {
		return b[:cryptoutil.AESKeySize]
	}
	h := sha256.Sum256(b)
	return h[:]
}

func EncryptStringWithAppKey(plain string) (string, error) {
	key := appKeyBytes()
	ct, err := cryptoutil.EncryptMessage([]byte(plain), key)
	if err != nil {
		return "", err
	}
	return "enc:" + base64.StdEncoding.EncodeToString(ct), nil
}

func DecryptStringWithAppKey(data string) (string, error) {
	if !strings.HasPrefix(data, "enc:") {
		return data, nil
	}
	enc := strings.TrimPrefix(data, "enc:")
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	key := appKeyBytes()
	pt, err := cryptoutil.DecryptMessage(raw, key)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
