package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAndUpdateEnvValuePreserveUnrelatedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	initial := "# comment\nAPP_HOST='127.0.0.1'\nAPP_PORT = 8080\nEMPTY=\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	host, err := ReadEnvValue(path, "APP_HOST")
	if err != nil || host != "127.0.0.1" {
		t.Fatalf("host=%q err=%v", host, err)
	}
	if err := UpdateEnvValue(path, "APP_PORT", "9000"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateEnvValue(path, "NEW_KEY", "value"); err != nil {
		t.Fatal(err)
	}
	port, err := ReadEnvValue(path, "APP_PORT")
	if err != nil || port != "9000" {
		t.Fatalf("port=%q err=%v", port, err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, "# comment") || !strings.Contains(text, "NEW_KEY=value") {
		t.Fatalf("env=%s", text)
	}
	if _, err := ReadEnvValue(path, "MISSING"); err == nil {
		t.Fatal("expected missing key error")
	}
}

func TestGetEnvFilePathPrefersWorkingDirectory(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(old) }()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("A=B"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	got, err := GetEnvFilePath()
	if err != nil || got != filepath.Join(dir, ".env") {
		t.Fatalf("path=%q err=%v", got, err)
	}
}

func TestUpdatePortInEnvUpdatesPortAndURL(t *testing.T) {
	old, _ := os.Getwd()
	defer func() { _ = os.Chdir(old) }()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("APP_HOST=0.0.0.0\nAPP_PORT=8080\nAPP_URL=http://old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := UpdatePortInEnv(12345); err != nil {
		t.Fatal(err)
	}
	port, _ := ReadEnvValue(path, "APP_PORT")
	appURL, _ := ReadEnvValue(path, "APP_URL")
	if port != "12345" || appURL != "http://0.0.0.0:12345" {
		t.Fatalf("port=%s url=%s", port, appURL)
	}
}

func TestGenerateRandomStringRespectsLengthAndCharacterSets(t *testing.T) {
	for _, tc := range []struct {
		length  int
		charset string
		allowed string
	}{
		{64, "alphanumeric", "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"},
		{64, "alphanumeric_special", "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*"},
		{64, "unknown", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"},
	} {
		got, err := GenerateRandomString(tc.length, tc.charset)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != tc.length {
			t.Fatalf("length=%d", len(got))
		}
		for _, r := range got {
			if !strings.ContainsRune(tc.allowed, r) {
				t.Fatalf("unexpected char %q", r)
			}
		}
	}
	empty, err := GenerateRandomString(0, "alphanumeric")
	if err != nil || empty != "" {
		t.Fatalf("empty=%q err=%v", empty, err)
	}
}

func TestGenerateRandomPortStaysWithinConfiguredRange(t *testing.T) {
	oldMin, oldMax := MinPort, MaxPort
	MinPort, MaxPort = 49152, 49153
	defer func() { MinPort, MaxPort = oldMin, oldMax }()
	port, err := GenerateRandomPort()
	if err != nil {
		t.Fatal(err)
	}
	if port < MinPort || port > MaxPort {
		t.Fatalf("port=%d", port)
	}
}
