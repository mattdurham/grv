// Namespace: goast/ops
// Git churn computation — counts commits that touched a given line range.
package ops

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultRepoRoot is set once by the daemon at startup. Empty = no git churn.
var DefaultRepoRoot string

// SetDefaultRepoRoot sets the repository root used for git churn queries.
func SetDefaultRepoRoot(root string) {
	DefaultRepoRoot = root
}

type churnKey struct {
	absFile string
	hash    string
	start   int
	end     int
}

var churnCache sync.Map // churnKey → int

// gitChurn returns the number of commits that touched lines [start, end] in absFile.
// Results are cached by (file, HEAD hash, start, end). Returns 0 when git is unavailable.
func gitChurn(absFile string, start, end int) int {
	if DefaultRepoRoot == "" || start <= 0 || end < start {
		return 0
	}
	hash := gitHeadHash(DefaultRepoRoot)
	if hash == "" {
		return 0
	}
	key := churnKey{absFile: absFile, hash: hash, start: start, end: end}
	if v, ok := churnCache.Load(key); ok {
		return v.(int)
	}

	relPath, err := filepath.Rel(DefaultRepoRoot, absFile)
	if err != nil {
		return 0
	}

	lineRange := fmt.Sprintf("-L%d,%d:%s", start, end, filepath.ToSlash(relPath))
	out, err := exec.Command("git", "-C", DefaultRepoRoot, "log", "--oneline", "--no-patch", lineRange).Output()
	count := 0
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				count++
			}
		}
	}
	churnCache.Store(key, count)
	return count
}

func gitHeadHash(repoRoot string) string {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
