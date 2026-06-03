package hooks

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Expand replaces {file}, {dir}, {repo_name}, {repo_path}, {namespace}, {pkg}, {project}
// in each arg. Also expands a leading ~/ to the home directory.
// Unknown placeholders are left unchanged.
func Expand(args []string, vars map[string]string) []string {
	home, _ := os.UserHomeDir()
	result := make([]string, len(args))
	for i, arg := range args {
		if home != "" && strings.HasPrefix(arg, "~/") {
			arg = home + "/" + arg[2:]
		}
		for k, v := range vars {
			arg = strings.ReplaceAll(arg, "{"+k+"}", v)
		}
		result[i] = arg
	}
	return result
}

// CollectVars gathers substitution values for the given absolute file path.
// repoRoot may be empty string (git unavailable).
func CollectVars(absFile, repoRoot string) map[string]string {
	vars := map[string]string{
		"file":      absFile,
		"dir":       filepath.Dir(absFile),
		"repo_path": repoRoot,
		"repo_name": "",
		"pkg":       scanPackageName(absFile),
		"namespace": hookPackageImportPath(filepath.Dir(absFile)),
		"project":   hookModuleName(filepath.Dir(absFile)),
	}
	if repoRoot != "" {
		vars["repo_name"] = filepath.Base(repoRoot)
	}
	return vars
}

// scanPackageName reads the first "package X" line from a Go file.
func scanPackageName(absFile string) string {
	f, err := os.Open(absFile)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "package ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

// hookPackageImportPath finds the Go module import path for a directory by
// reading the nearest go.mod and combining module + relative path.
// Duplicated from ops/place.go to avoid a circular import (ops imports hooks).
func hookPackageImportPath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	for d := abs; d != filepath.Dir(d); d = filepath.Dir(d) {
		gomod := filepath.Join(d, "go.mod")
		data, err := os.ReadFile(gomod)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				mod := strings.TrimSpace(strings.TrimPrefix(line, "module"))
				rel, _ := filepath.Rel(d, abs)
				if rel == "." || rel == "" {
					return mod
				}
				return mod + "/" + filepath.ToSlash(rel)
			}
		}
	}
	return ""
}

// hookModuleName returns the bare module name from the nearest go.mod
// (e.g. "github.com/grafana/tempo"), without any subpackage path appended.
func hookModuleName(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	for d := abs; d != filepath.Dir(d); d = filepath.Dir(d) {
		gomod := filepath.Join(d, "go.mod")
		data, err := os.ReadFile(gomod)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "module"))
			}
		}
	}
	return ""
}
