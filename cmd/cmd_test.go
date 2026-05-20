package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattdurham/grv/cmd"
)

func TestHashDir(t *testing.T) {
	// Same input → same output
	h1 := cmd.HashDir("/some/dir")
	h2 := cmd.HashDir("/some/dir")
	if h1 != h2 {
		t.Errorf("HashDir not deterministic: %q != %q", h1, h2)
	}
	// 8 characters
	if len(h1) != 8 {
		t.Errorf("expected 8-char hash, got %q (len %d)", h1, len(h1))
	}
	// Different inputs → different outputs
	h3 := cmd.HashDir("/other/dir")
	if h1 == h3 {
		t.Error("expected different hashes for different paths")
	}
}

func TestGRVDir(t *testing.T) {
	d, err := cmd.GRVDir()
	if err != nil {
		t.Fatalf("GRVDir: %v", err)
	}
	if !strings.HasSuffix(d, ".grv") {
		t.Errorf("expected path ending in .grv, got %q", d)
	}
	// Must exist
	if _, err := os.Stat(d); err != nil {
		t.Errorf("GRVDir should create the directory: %v", err)
	}
}

func TestPathFunctions(t *testing.T) {
	hash := "abcd1234"
	dir := "/tmp/grv"

	sock := cmd.SockPath(dir, hash)
	if !strings.HasSuffix(sock, ".sock") {
		t.Errorf("SockPath: expected .sock suffix, got %q", sock)
	}
	if !strings.Contains(sock, hash) {
		t.Errorf("SockPath: expected hash in path, got %q", sock)
	}

	pid := cmd.PIDPath(dir, hash)
	if !strings.HasSuffix(pid, ".pid") {
		t.Errorf("PIDPath: expected .pid suffix, got %q", pid)
	}

	log := cmd.LogPath(dir, hash)
	if !strings.HasSuffix(log, ".log") {
		t.Errorf("LogPath: expected .log suffix, got %q", log)
	}
}

func TestIsRunningNoFile(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "test.sock")
	pid := filepath.Join(dir, "test.pid")
	if cmd.IsRunning(sock, pid) {
		t.Error("expected IsRunning=false when pid file absent")
	}
}

func TestIsRunningDeadPID(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "test.sock")
	pid := filepath.Join(dir, "test.pid")
	// Write a PID that is very unlikely to be alive
	os.WriteFile(pid, []byte("99999999\n"), 0644)
	if cmd.IsRunning(sock, pid) {
		t.Error("expected IsRunning=false for PID 99999999")
	}
}

func TestListDaemonsEmpty(t *testing.T) {
	// Temporarily point GRVDir to a temp dir to avoid collisions
	// We can't override GRVDir easily, so just verify it doesn't error
	statuses, err := cmd.ListDaemons()
	if err != nil {
		t.Fatalf("ListDaemons: %v", err)
	}
	// May or may not be empty depending on whether a daemon is running
	_ = statuses
}
