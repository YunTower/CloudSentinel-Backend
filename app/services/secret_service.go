package services

import (
	"goravel/app/utils/secret"
)

func EncryptStringWithAppKey(plain string) (string, error) {
	return secret.EncryptStringWithAppKey(plain)
}

func DecryptStringWithAppKey(data string) (string, error) {
	return secret.DecryptStringWithAppKey(data)
}
