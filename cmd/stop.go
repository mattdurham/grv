package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// StopDaemon sends SIGTERM to the daemon for dir.
func StopDaemon(dir string) error {
	grvDir, err := GRVDir()
	if err != nil {
		return fmt.Errorf("grv dir: %w", err)
	}
	hash := HashDir(dir)
	pidPath := PIDPath(grvDir, hash)
	sockPath := SockPath(grvDir, hash)

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("read pid file: %w", err)
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("parse pid: %w", err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	proc.Signal(syscall.SIGTERM)

	os.Remove(pidPath)
	os.Remove(sockPath)
	return nil
}
