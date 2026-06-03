package hooks

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ChecksConfig holds rule enforcement settings loaded from grv.yaml [checks].
type ChecksConfig struct {
	Enforce []string `yaml:"enforce"` // rule names to enforce, or ["all"] for every built-in rule
}

// HookConfig describes a single configured hook.
type HookConfig struct {
	Name        string        `yaml:"name"`
	Command     []string      `yaml:"command"`
	Scope       string        `yaml:"scope"`        // "file" | "all"
	Cache       bool          `yaml:"cache"`
	Immutable   bool          `yaml:"immutable"`
	TimeoutStr  string        `yaml:"timeout"`      // e.g. "10s" — parsed into Timeout
	Timeout     time.Duration `yaml:"-"`
	Kinds       []string      `yaml:"kinds"`        // optional kind filter
	StripFields []string      `yaml:"strip_fields"` // JSON keys to remove from each result before storing
}

// configFile is the top-level structure of grv.yaml.
type configFile struct {
	Hooks  []HookConfig `yaml:"hooks"`
	Checks ChecksConfig  `yaml:"checks"`
}

// LoadConfig searches for a grv.yaml config file starting at dir, then walking
// up to the go.mod root, then checking ~/.grv/config.yaml.
// Returns nil configs and zero ChecksConfig if no config file is found.
func LoadConfig(dir string) ([]HookConfig, ChecksConfig, error) {
	path := findConfigFile(dir)
	if path == "" {
		return nil, ChecksConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ChecksConfig{}, err
	}
	return parseConfig(data)
}

// findConfigFile returns the first config file found: grv.yaml in dir, then
// walking up to the go.mod root, then ~/.grv/config.yaml. Returns "" if not found.
func findConfigFile(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}

	// Check dir for grv.yaml
	if _, err := os.Stat(filepath.Join(abs, "grv.yaml")); err == nil {
		return filepath.Join(abs, "grv.yaml")
	}

	// Walk up looking for go.mod, then check there
	for d := filepath.Dir(abs); d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(d, "grv.yaml")); err == nil {
				return filepath.Join(d, "grv.yaml")
			}
			break
		}
	}

	// Check ~/.grv/config.yaml
	if home, err := os.UserHomeDir(); err == nil {
		if p := filepath.Join(home, ".grv", "config.yaml"); fileExists(p) {
			return p
		}
	}

	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// parseConfig unmarshals YAML config data.
func parseConfig(data []byte) ([]HookConfig, ChecksConfig, error) {
	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, ChecksConfig{}, err
	}

	// Parse timeout strings and apply defaults.
	hooks := cfg.Hooks[:0:len(cfg.Hooks)]
	for _, h := range cfg.Hooks {
		if h.Name == "" {
			continue
		}
		if h.TimeoutStr != "" {
			if d, err := time.ParseDuration(h.TimeoutStr); err == nil {
				h.Timeout = d
			}
		}
		if h.Timeout == 0 {
			h.Timeout = 5 * time.Second
		}
		hooks = append(hooks, h)
	}

	return hooks, cfg.Checks, nil
}
