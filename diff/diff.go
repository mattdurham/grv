// Namespace: goast/diff
// Package diff wraps go-difflib to produce unified diffs between file versions.
package diff

import (
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// Files produces a unified diff between before and after bytes, labelled with path.
// Returns empty string if no changes.
func Files(path string, before, after []byte) (string, error) {
	if string(before) == string(after) {
		return "", nil
	}
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(before)),
		B:        difflib.SplitLines(string(after)),
		FromFile: path,
		ToFile:   path,
		Context:  3,
	}
	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(result, "\n"), nil
}
