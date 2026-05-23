package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// RunnerInterface defines the hook runner contract for ops package injection.
// Defined here so hooks.Runner satisfies it without importing ops.
type RunnerInterface interface {
	RunFile(absFile string) map[string]interface{}
	Invalidate(absFile string)
}

// Runner executes configured hooks for a file and maintains a result cache.
//
// Concurrent RunFile calls may run the subprocess more than once if neither
// call has cached a result yet; results are consistent because hooks are idempotent.
type Runner struct {
	configs  []HookConfig
	cache    *Cache
	repoRoot string
}

// New creates a Runner with the given hook configs and repository root path.
func New(configs []HookConfig, repoRoot string) *Runner {
	return &Runner{
		configs:  configs,
		cache:    NewCache(),
		repoRoot: repoRoot,
	}
}

// RunFile runs all file-scope hooks for absFile and returns a merged result map.
// Keys are prefixed with the hook name: "hookname.field".
// On subprocess error, sets "hookname.error" instead of data keys.
func (r *Runner) RunFile(absFile string) map[string]interface{} {
	result := make(map[string]interface{})

	for _, cfg := range r.configs {
		if cfg.Scope != "file" && cfg.Scope != "all" {
			continue
		}

		if cfg.Cache {
			if cached, ok := r.cache.Get(cfg.Name, absFile); ok {
				for k, v := range cached {
					result[cfg.Name+"."+k] = v
				}
				continue
			}
		}

		vars := CollectVars(absFile, r.repoRoot)
		expanded := Expand(cfg.Command, vars)

		if len(expanded) == 0 {
			result[cfg.Name+".error"] = "empty command"
			continue
		}

		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 5 * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, expanded[0], expanded[1:]...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		cancel()

		if runErr != nil {
			result[cfg.Name+".error"] = fmt.Sprintf("%v", runErr)
			continue
		}

		if stdout.Len() == 0 {
			continue
		}

		hookResult, err := parseHookOutput(stdout.Bytes())
		if err != nil {
			result[cfg.Name+".error"] = "json: " + err.Error()
			continue
		}

		for k, v := range hookResult {
			result[cfg.Name+"."+k] = v
		}

		if cfg.Cache {
			if fi, err := os.Stat(absFile); err == nil {
				r.cache.Set(cfg.Name, absFile, hookResult, fi.ModTime())
			}
		}
	}

	return result
}

// Invalidate clears all cached results for absFile.
func (r *Runner) Invalidate(absFile string) {
	r.cache.Invalidate(absFile)
}

// parseHookOutput unmarshals hook stdout into a flat map.
// If the output is a JSON array, it is wrapped as {"results": [...]}.
func parseHookOutput(b []byte) (map[string]interface{}, error) {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil, nil
	}
	if b[0] == '[' {
		var arr []interface{}
		if err := json.Unmarshal(b, &arr); err != nil {
			return nil, err
		}
		return map[string]interface{}{"results": arr}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}
