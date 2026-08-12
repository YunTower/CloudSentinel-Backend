package websocket

import (
	"bytes"
	"testing"
	"time"
)

func TestConnectionStateStringAndDefaultConfig(t *testing.T) {
	if StateConnecting.String() != "connecting" || StateAuthenticated.String() != "authenticated" || StateClosed.String() != "closed" || ConnectionState(99).String() != "unknown" {
		t.Fatal("connection state mapping mismatch")
	}
	cfg := DefaultConfig()
	if cfg.ReadBufferSize != 1024 || cfg.WriteBufferSize != 1024 || cfg.ReadTimeout != 90*time.Second || cfg.WriteTimeout != 10*time.Second || cfg.MaxMessageSize != 512 {
		t.Fatalf("defaults=%+v", cfg)
	}
	if NewUpgrader(nil) == nil || NewUpgrader(cfg).config != cfg {
		t.Fatal("upgrader config mismatch")
	}
}

func TestUpgraderExtractsIPv4IPv6AndUnknownAddresses(t *testing.T) {
	u := NewUpgrader(nil)
	tests := map[string]string{
		"127.0.0.1:8080": "127.0.0.1", "[2001:db8::1]:443": "2001:db8::1",
		"hostname": "hostname", "": "unknown",
	}
	for input, want := range tests {
		if got := u.ExtractIPFromAddrString(input); got != want {
			t.Errorf("%q => %q want %q", input, got, want)
		}
	}
	if got := u.ExtractIPFromAddr(nil); got != "unknown" {
		t.Fatalf("nil => %q", got)
	}
}

func TestBaseConnectionStateAndPingWithoutSocketIO(t *testing.T) {
	c := NewBaseConnection(nil, DefaultConfig())
	if c.GetState() != StateConnecting || c.IsClosed() {
		t.Fatal("initial state mismatch")
	}
	before := c.GetLastPing()
	time.Sleep(time.Millisecond)
	c.UpdateLastPing()
	if !c.GetLastPing().After(before) {
		t.Fatal("ping was not updated")
	}
	c.SetState(StateAuthenticated)
	if c.GetState() != StateAuthenticated {
		t.Fatal("state not updated")
	}
}

func TestAgentConnectionMetadataAndSessionKeyAreCopied(t *testing.T) {
	c := NewAgentConnection(nil, DefaultConfig())
	c.SetServerID("server-1")
	c.SetAgentKey("secret")
	c.SetRemoteAddr("1.2.3.4")
	c.SetAgentPublicKey("pub")
	c.SetAgentFingerprint("fingerprint")
	key := bytes.Repeat([]byte{7}, 32)
	c.SetSessionKey(key)
	key[0] = 9
	c.EnableEncryption()
	got := c.GetSessionKey()
	got[0] = 1
	if c.GetSessionKey()[0] != 7 {
		t.Fatal("session key leaked mutable storage")
	}
	info := c.GetInfo()
	info.SessionKey[0] = 2
	if c.GetSessionKey()[0] != 7 || info.ServerID != "server-1" || info.AgentKey != "secret" || info.RemoteAddr != "1.2.3.4" || info.AgentPublicKey != "pub" || info.AgentFingerprint != "fingerprint" || !info.EncryptionEnabled {
		t.Fatalf("info=%+v", info)
	}
	c.SetSessionKey(nil)
	if c.GetSessionKey() != nil {
		t.Fatal("nil key not cleared")
	}
}

func TestFrontendGuestAllowlistDoesNotRestrictAuthenticatedUsers(t *testing.T) {
	c := NewFrontendConnection(nil, DefaultConfig())
	c.SetConnID("conn")
	c.SetUserID("user")
	c.SetRemoteAddr("1.2.3.4")
	c.SetUserType("guest")
	c.SetAllowedServerIDs(true, []string{"server-1", "", "server-1"})
	if !c.CanAccessServer("server-1") || c.CanAccessServer("server-2") || c.CanAccessServer("") {
		t.Fatal("guest allowlist mismatch")
	}
	c.SetAllowedServerIDs(false, nil)
	if !c.CanAccessServer("server-2") {
		t.Fatal("unrestricted guest should access")
	}
	c.SetAllowedServerIDs(true, nil)
	c.SetUserType("admin")
	if !c.CanAccessServer("server-2") {
		t.Fatal("admin should bypass guest allowlist")
	}
	info := c.GetInfo()
	if info.ConnID != "conn" || info.UserID != "user" || info.RemoteAddr != "1.2.3.4" || info.UserType != "admin" {
		t.Fatalf("info=%+v", info)
	}
}

func TestWebSocketAESHelpersRoundTripAndRejectBadInputs(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 32)
	plain := []byte("payload")
	ciphertext, err := encryptMessageAES(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decryptMessageAES(ciphertext, key)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := encryptMessageAES(plain, []byte("short")); err == nil {
		t.Fatal("expected key error")
	}
	if _, err := decryptMessageAES([]byte("short"), key); err == nil {
		t.Fatal("expected short ciphertext error")
	}
	ciphertext[len(ciphertext)-1] ^= 0xff
	if _, err := decryptMessageAES(ciphertext, key); err == nil {
		t.Fatal("expected tamper error")
	}
}
