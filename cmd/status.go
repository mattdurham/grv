package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DaemonStatus describes one running (or dead) daemon.
type DaemonStatus struct {
	Hash     string
	PIDPath  string
	SockPath string
	Alive    bool
	PID      int
}

// ListDaemons returns status for all known daemons in ~/.grv/.
func ListDaemons() ([]DaemonStatus, error) {
	grvDir, err := GRVDir()
	if err != nil {
		return nil, fmt.Errorf("grv dir: %w", err)
	}

	entries, err := os.ReadDir(grvDir)
	if err != nil {
		return nil, fmt.Errorf("read grv dir: %w", err)
	}

	var statuses []DaemonStatus
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pid") {
			continue
		}
		hash := strings.TrimSuffix(entry.Name(), ".pid")
		pidPath := filepath.Join(grvDir, entry.Name())
		sockPath := SockPath(grvDir, hash)

		data, err := os.ReadFile(pidPath)
		if err != nil {
			continue
		}
		pidStr := strings.TrimSpace(string(data))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		alive := false
		if proc, err := os.FindProcess(pid); err == nil {
			alive = proc.Signal(syscall.Signal(0)) == nil
		}

		statuses = append(statuses, DaemonStatus{
			Hash:     hash,
			PIDPath:  pidPath,
			SockPath: sockPath,
			Alive:    alive,
			PID:      pid,
		})
	}
	return statuses, nil
}

// PrintStatus prints all daemon statuses to stdout.
func PrintStatus() error {
	statuses, err := ListDaemons()
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		fmt.Println("no grv daemons running")
		return nil
	}
	for _, s := range statuses {
		state := "dead"
		if s.Alive {
			state = fmt.Sprintf("running (pid %d)", s.PID)
		}
		fmt.Printf("hash=%-8s  %-24s  sock=%s\n", s.Hash, state, s.SockPath)
	}
	return nil
}
