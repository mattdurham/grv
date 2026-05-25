package hooks

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HookConfig struct

// HookConfig describes a single configured hook.
{
	Name      string
	Command   []string
	Scope     string
	Cache     bool
	Immutable bool
	Timeout   time.Duration
	Kinds     []string
}

// "file" | "all"

// default 5s if zero after parsing
// optional kind filter

// LoadConfig searches for a goast.toml config file starting at dir, then walking
// up to the go.mod root, then checking ~/.grv/config.toml.
// Returns nil, nil if no config file is found (config is optional).
func LoadConfig(dir string) ([]HookConfig, error) {
	path := findConfigFile(dir)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseGRVTOML(data)
}

// findConfigFile returns the first goast.toml found: dir/goast.toml, then
// walking up to the go.mod root, then ~/.grv/config.toml. Returns "" if not found.
func findConfigFile(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}

	// Check dir for grv.toml or goast.toml (grv.toml takes precedence)
	for _, name := range []string{"grv.toml", "goast.toml"} {
		candidate := filepath.Join(abs, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Walk up looking for go.mod, then check grv.toml/goast.toml there
	for d := filepath.Dir(abs); d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			for _, name := range []string{"grv.toml", "goast.toml"} {
				candidate := filepath.Join(d, name)
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
			break
		}
	}

	// Check ~/.grv/config.toml
	if home, err := os.UserHomeDir(); err == nil {
		globalConfig := filepath.Join(home, ".grv", "config.toml")
		if _, err := os.Stat(globalConfig); err == nil {
			return globalConfig
		}
	}

	return ""
}

// parseGRVTOML parses a TOML file containing [[hooks]] array-of-tables.
func parseGRVTOML(data []byte) ([]HookConfig, error) {
	var configs []HookConfig
	var current *HookConfig

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if line == "[[hooks]]" {
			if current != nil && current.Name != "" {
				configs = append(configs, *current)
			}
			current = &HookConfig{}
			continue
		}

		if current == nil {
			continue
		}

		key, value, ok := parseLine(line)
		if !ok {
			continue
		}

		switch key {
		case "immutable":
			current.Immutable = strings.TrimSpace(value) == "true"
		case "name":
			current.Name = unquote(value)
		case "command":
			current.Command = parseInlineArray(value)
		case "scope":
			current.Scope = unquote(value)
		case "cache":
			current.Cache = strings.TrimSpace(value) == "true"
		case "timeout":
			d, err := time.ParseDuration(unquote(value))
			if err != nil {
				d = 5 * time.Second
			}
			current.Timeout = d
		case "kinds":
			current.Kinds = parseInlineArray(value)
		}
	}

	if current != nil && current.Name != "" {
		configs = append(configs, *current)
	}

	// Apply default timeout
	for i := range configs {
		if configs[i].Timeout == 0 {
			configs[i].Timeout = 5 * time.Second
		}
	}

	return configs, scanner.Err()
}

// parseLine splits "key = value" into (key, value, true). Returns (_, _, false) on failure.
func parseLine(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// unquote strips leading/trailing double-quotes, whitespace, and unescapes \" → "
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, `\"`, `"`)
		s = strings.ReplaceAll(s, `\\`, `\`)
	}
	return s
}

// parseInlineArray parses a TOML inline array like ["a", "b c", "d"].
// Handles quoted strings containing commas correctly.
func parseInlineArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	s = s[1 : len(s)-1] // strip [ ]
	var result []string
	for len(s) > 0 {
		s = strings.TrimSpace(s)
		if len(s) == 0 {
			break
		}
		if s[0] == '"' {
			// find matching closing quote, respecting \"
			end := 1
			for end < len(s) {
				if s[end] == '\\' {
					end += 2
					continue
				}
				if s[end] == '"' {
					break
				}
				end++
			}
			result = append(result, unquote(s[:end+1]))
			s = strings.TrimSpace(s[end+1:])
			if len(s) > 0 && s[0] == ',' {
				s = s[1:]
			}
		} else {
			// unquoted value — find next comma
			idx := strings.IndexByte(s, ',')
			if idx < 0 {
				v := strings.TrimSpace(s)
				if v != "" {
					result = append(result, v)
				}
				break
			}
			v := strings.TrimSpace(s[:idx])
			if v != "" {
				result = append(result, v)
			}
			s = s[idx+1:]
		}
	}
	return result
}
