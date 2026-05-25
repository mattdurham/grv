package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// DaemonStatus describes the running (or dead) daemon.
type DaemonStatus struct {
	PIDPath  string
	SockPath string
	Alive    bool
	PID      int
}

// ListDaemons returns status for the single user-level daemon.
func ListDaemons() ([]DaemonStatus, error) {
	grvDir, err := GRVDir()
	if err != nil {
		return nil, fmt.Errorf("grv dir: %w", err)
	}

	pidPath := PIDPath(grvDir)
	sockPath := SockPath(grvDir)

	data, err := os.ReadFile(pidPath)
	if err != nil {
		return nil, nil // no daemon ever started
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, nil
	}

	alive := false
	if proc, err := os.FindProcess(pid); err == nil {
		alive = proc.Signal(syscall.Signal(0)) == nil
	}

	return []DaemonStatus{{
		PIDPath:  pidPath,
		SockPath: sockPath,
		Alive:    alive,
		PID:      pid,
	}}, nil
}

// PrintStatus prints daemon status to stdout.
func PrintStatus() error {
	statuses, err := ListDaemons()
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		fmt.Println("no grv daemon running")
		return nil
	}
	s := statuses[0]
	state := "dead"
	if s.Alive {
		state = fmt.Sprintf("running (pid %d)", s.PID)
	}
	fmt.Printf("%-24s  sock=%s\n", state, s.SockPath)
	return nil
}
