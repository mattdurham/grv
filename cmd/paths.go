package cmd

import (
	"crypto/sha256"
	"fmt"
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

// HashDir returns an 8-character hex string derived from the absolute directory path.
func HashDir(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("%x", h[:4])
}

func SockPath(grvDir, hash string) string { return filepath.Join(grvDir, hash+".sock") }
func PIDPath(grvDir, hash string) string  { return filepath.Join(grvDir, hash+".pid") }
func LogPath(grvDir, hash string) string  { return filepath.Join(grvDir, hash+".log") }
