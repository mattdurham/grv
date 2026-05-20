package diff

import (
	"strings"
	"testing"
)

func TestFilesIdentical(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")
	result, err := Files("test.go", content, content)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty diff for identical content, got: %q", result)
	}
}

func TestFilesAddedLine(t *testing.T) {
	before := []byte("line1\nline2\n")
	after := []byte("line1\nline2\nline3\n")
	result, err := Files("test.go", before, after)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "+line3") {
		t.Errorf("expected +line3 in diff, got: %q", result)
	}
}

func TestFilesRemovedLine(t *testing.T) {
	before := []byte("line1\nline2\nline3\n")
	after := []byte("line1\nline3\n")
	result, err := Files("test.go", before, after)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "-line2") {
		t.Errorf("expected -line2 in diff, got: %q", result)
	}
}

func TestFilesChangedLine(t *testing.T) {
	before := []byte("line1\nold\nline3\n")
	after := []byte("line1\nnew\nline3\n")
	result, err := Files("test.go", before, after)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "-old") || !strings.Contains(result, "+new") {
		t.Errorf("expected -old and +new in diff, got: %q", result)
	}
	if !strings.Contains(result, "line1") {
		t.Errorf("expected context line in diff, got: %q", result)
	}
}
