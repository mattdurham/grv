package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// IsRunning returns true if a daemon is alive at pidPath/sockPath.
func IsRunning(sockPath, pidPath string) bool {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// StartDaemon ensures a daemon is running for dir.
func StartDaemon(dir string) error {
	grvDir, err := GRVDir()
	if err != nil {
		return fmt.Errorf("grv dir: %w", err)
	}
	hash := HashDir(dir)
	sockPath := SockPath(grvDir, hash)
	pidPath := PIDPath(grvDir, hash)
	logPath := LogPath(grvDir, hash)

	if IsRunning(sockPath, pidPath) {
		return nil
	}

	// Clean up stale files
	os.Remove(sockPath)
	os.Remove(pidPath)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("eval symlinks: %w", err)
	}

	logF, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}

	cmd := exec.Command(exe, "daemon", "--socket", sockPath, "--pid", pidPath, "--log", logPath, "--dir", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdout = logF
	cmd.Stderr = logF

	if err := cmd.Start(); err != nil {
		logF.Close()
		return fmt.Errorf("start daemon: %w", err)
	}
	logF.Close()

	// Wait up to 2s for socket to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sockPath); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not start within 2 seconds")
}
