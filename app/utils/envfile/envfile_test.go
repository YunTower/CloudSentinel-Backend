package envfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAndUpdatePreserveUnrelatedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	initial := "# comment\nAPP_HOST='127.0.0.1'\nAPP_PORT = 8080\nEMPTY=\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	host, err := Read(path, "APP_HOST")
	if err != nil || host != "127.0.0.1" {
		t.Fatalf("host=%q err=%v", host, err)
	}
	changed, err := Update(path, "APP_PORT", "9000")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	changed, err = Update(path, "NEW_KEY", "value")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	// 相同值重复写入不产生变更
	changed, err = Update(path, "NEW_KEY", "value")
	if err != nil || changed {
		t.Fatalf("no-op update: changed=%v err=%v", changed, err)
	}
	port, err := Read(path, "APP_PORT")
	if err != nil || port != "9000" {
		t.Fatalf("port=%q err=%v", port, err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, "# comment") || !strings.Contains(text, "NEW_KEY=value") {
		t.Fatalf("env=%s", text)
	}
	if _, err := Read(path, "MISSING"); err == nil {
		t.Fatal("expected missing key error")
	}
}

func TestPathPrefersWorkingDirectory(t *testing.T) {
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
	got, err := Path()
	if err != nil || got != filepath.Join(dir, ".env") {
		t.Fatalf("path=%q err=%v", got, err)
	}
}

func TestUpdateMissingFileReturnsError(t *testing.T) {
	if _, err := Update(filepath.Join(t.TempDir(), ".env"), "A", "B"); err == nil {
		t.Fatal("expected error for missing file")
	}
}
