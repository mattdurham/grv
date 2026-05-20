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

// ---- convert ----

func TestBuildConvertResult_Basic(t *testing.T) {
	// BuildConvertResult processes ast_directory JSON into a ConvertResult.
	// We feed it a synthetic directory listing and check the counts.
	input := `{
		"go_files": [
			{"file":"main.go","readonly":false,"package":"main","structs":[],"functions":[{},{}]},
			{"file":"vendor/pkg/lib.go","readonly":true,"package":"lib","structs":[{}],"functions":[{}]}
		],
		"non_go_files": [
			{"file":"README.md","readonly":false,"size":1024},
			{"file":"go.mod","readonly":false,"size":256}
		],
		"subdirs": ["cmd","daemon"]
	}`

	result, err := cmd.BuildConvertResult([]byte(input))
	if err != nil {
		t.Fatalf("BuildConvertResult: %v", err)
	}

	if result.ReadWrite != 3 { // main.go + README.md + go.mod
		t.Errorf("ReadWrite: expected 3, got %d", result.ReadWrite)
	}
	if result.ReadOnly != 1 { // vendor/pkg/lib.go
		t.Errorf("ReadOnly: expected 1, got %d", result.ReadOnly)
	}
	if len(result.GoFiles) != 2 {
		t.Errorf("GoFiles: expected 2, got %d", len(result.GoFiles))
	}
	if len(result.NonGoFiles) != 2 {
		t.Errorf("NonGoFiles: expected 2, got %d", len(result.NonGoFiles))
	}
	if len(result.Subdirs) != 2 {
		t.Errorf("Subdirs: expected 2, got %d", len(result.Subdirs))
	}
}

func TestBuildConvertResult_EmptyDir(t *testing.T) {
	input := `{"go_files":[],"non_go_files":[],"subdirs":[]}`
	result, err := cmd.BuildConvertResult([]byte(input))
	if err != nil {
		t.Fatalf("BuildConvertResult: %v", err)
	}
	if result.ReadWrite != 0 || result.ReadOnly != 0 {
		t.Errorf("expected 0 files, got rw=%d ro=%d", result.ReadWrite, result.ReadOnly)
	}
}

func TestBuildConvertResult_AllReadOnly(t *testing.T) {
	input := `{
		"go_files": [
			{"file":"std/fmt/print.go","readonly":true,"package":"fmt","structs":[],"functions":[]}
		],
		"non_go_files": [],
		"subdirs": []
	}`
	result, err := cmd.BuildConvertResult([]byte(input))
	if err != nil {
		t.Fatalf("BuildConvertResult: %v", err)
	}
	if result.ReadOnly != 1 {
		t.Errorf("expected 1 readonly, got %d", result.ReadOnly)
	}
	if result.ReadWrite != 0 {
		t.Errorf("expected 0 readwrite, got %d", result.ReadWrite)
	}
}

func TestConvertReport_ContainsKeyInfo(t *testing.T) {
	// ConvertReport produces a string that mentions file counts and status markers.
	input := `{
		"go_files": [
			{"file":"main.go","readonly":false,"package":"main","structs":[{}],"functions":[{},{},{}]}
		],
		"non_go_files": [
			{"file":"config.yaml","readonly":false,"size":512}
		],
		"subdirs": ["internal"]
	}`
	result, err := cmd.BuildConvertResult([]byte(input))
	if err != nil {
		t.Fatalf("BuildConvertResult: %v", err)
	}
	report := cmd.FormatConvertReport("/some/dir", result, false)

	if !strings.Contains(report, "main.go") {
		t.Error("report should mention main.go")
	}
	if !strings.Contains(report, "config.yaml") {
		t.Error("report should mention config.yaml")
	}
	if !strings.Contains(report, "[rw]") {
		t.Error("report should show [rw] for writable files")
	}
	if !strings.Contains(report, "Read-write: 2") {
		t.Errorf("report should show Read-write: 2, got:\n%s", report)
	}
	if !strings.Contains(report, "internal") {
		t.Error("report should list subdirs")
	}
}
