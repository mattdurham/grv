package cmd

import (
	"os"
	"path/filepath"
)

// GRVDir returns ~/.grv, creating it if needed.
func GRVDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(home, ".grv")
	return d, os.MkdirAll(d, 0755)
}

func SockPath(grvDir string) string { return filepath.Join(grvDir, "grv.sock") }
func PIDPath(grvDir string) string  { return filepath.Join(grvDir, "grv.pid") }
func LogPath(grvDir string) string  { return filepath.Join(grvDir, "grv.log") }
