package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"goravel/app/utils/envfile"
)

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
	port, _ := envfile.Read(path, "APP_PORT")
	appURL, _ := envfile.Read(path, "APP_URL")
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
