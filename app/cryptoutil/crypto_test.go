package cryptoutil

import (
	"bytes"
	"testing"
)

func TestSessionKeyAndAESGCMRoundTrip(t *testing.T) {
	key, err := GenerateSessionKey()
	if err != nil || len(key) != AESKeySize {
		t.Fatalf("key length=%d err=%v", len(key), err)
	}
	plain := []byte("CloudSentinel report")
	one, err := EncryptMessage(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	two, err := EncryptMessage(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(one, two) {
		t.Fatal("nonces must make ciphertext unique")
	}
	got, err := DecryptMessage(one, key)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestAESRejectsInvalidKeyShortCiphertextAndTampering(t *testing.T) {
	if _, err := EncryptMessage([]byte("x"), []byte("short")); err == nil {
		t.Fatal("expected key error")
	}
	key := bytes.Repeat([]byte{1}, AESKeySize)
	if _, err := DecryptMessage([]byte("short"), key); err == nil {
		t.Fatal("expected short ciphertext error")
	}
	ciphertext, _ := EncryptMessage([]byte("x"), key)
	ciphertext[len(ciphertext)-1] ^= 0xff
	if _, err := DecryptMessage(ciphertext, key); err == nil {
		t.Fatal("expected authentication error")
	}
}

func TestRSAEncryptionBase64FingerprintAndSignature(t *testing.T) {
	privateKey, publicKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := GetPublicKeyFingerprint(publicKey)
	if err != nil || len(fingerprint) != 64 {
		t.Fatalf("fingerprint=%q err=%v", fingerprint, err)
	}
	plain := []byte("session-key")
	encrypted, err := EncryptWithPublicKey(plain, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecryptWithPrivateKey(encrypted, privateKey)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("got=%q err=%v", got, err)
	}
	encoded, err := EncryptWithPublicKeyBase64(plain, publicKey)
	if err != nil {
		t.Fatal(err)
	}
	got, err = DecryptWithPrivateKeyBase64(encoded, privateKey)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("got=%q err=%v", got, err)
	}
	sig, err := SignData(plain, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := VerifySignature(plain, sig, publicKey)
	if err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
	valid, err = VerifySignature([]byte("changed"), sig, publicKey)
	if err != nil || valid {
		t.Fatalf("tampered valid=%v err=%v", valid, err)
	}
}

func TestRSARejectsMalformedInput(t *testing.T) {
	bad := "not-pem"
	if _, err := GetPublicKeyFingerprint(bad); err == nil {
		t.Fatal("expected public key error")
	}
	if _, err := EncryptWithPublicKey([]byte("x"), bad); err == nil {
		t.Fatal("expected encrypt error")
	}
	if _, err := DecryptWithPrivateKey([]byte("x"), bad); err == nil {
		t.Fatal("expected decrypt error")
	}
	if _, err := SignData([]byte("x"), bad); err == nil {
		t.Fatal("expected sign error")
	}
	if _, err := VerifySignature([]byte("x"), []byte("x"), bad); err == nil {
		t.Fatal("expected verify error")
	}
	if _, err := DecryptWithPrivateKeyBase64("%%%", bad); err == nil {
		t.Fatal("expected base64 error")
	}
}
