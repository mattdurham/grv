package main

import (
	"encoding/json"
	"testing"
)

func TestParseToolFlags_NoPositional(t *testing.T) {
	result := parseToolFlags([]string{"--file", "foo.go"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["namespace"]; ok {
		t.Error("expected no 'namespace' key")
	}
	var file string
	if err := json.Unmarshal(m["file"], &file); err != nil || file != "foo.go" {
		t.Errorf("expected file=foo.go, got %s", file)
	}
}

func TestParseToolFlags_PositionalFirst(t *testing.T) {
	result := parseToolFlags([]string{"hooks", "--path", "[1,2]"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil || ns != "hooks" {
		t.Errorf("expected namespace=hooks, got %s", ns)
	}
	var path []int
	if err := json.Unmarshal(m["path"], &path); err != nil || len(path) != 2 {
		t.Errorf("expected path=[1,2], got %v", path)
	}
}

func TestParseToolFlags_PositionalOnly(t *testing.T) {
	result := parseToolFlags([]string{"hooks#RunFile"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil || ns != "hooks#RunFile" {
		t.Errorf("expected namespace=hooks#RunFile, got %s", ns)
	}
}

func TestParseToolFlags_OnlyFirstPositionalCaptured(t *testing.T) {
	result := parseToolFlags([]string{"hooks", "other"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil || ns != "hooks" {
		t.Errorf("expected namespace=hooks only, got %s", ns)
	}
}

func TestParseToolFlags_BoolFlagWithPositional(t *testing.T) {
	result := parseToolFlags([]string{"hooks", "--dry_run"})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	var ns string
	if err := json.Unmarshal(m["namespace"], &ns); err != nil || ns != "hooks" {
		t.Errorf("expected namespace=hooks, got %s", ns)
	}
	var dryRun bool
	if err := json.Unmarshal(m["dry_run"], &dryRun); err != nil || !dryRun {
		t.Error("expected dry_run=true")
	}
}

func TestParseToolFlags_Empty(t *testing.T) {
	result := parseToolFlags([]string{})
	var m map[string]json.RawMessage
	if err := json.Unmarshal(result, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func TestParseToolFlags_EmbeddedEquals(t *testing.T) {
	tests := []struct {
		args    []string
		key     string
		wantVal string
	}{
		{[]string{"--dry-run=somevalue"}, "dry_run", "somevalue"},
		{[]string{"--flag="}, "flag", ""},
	}
	for _, tc := range tests {
		result := parseToolFlags(tc.args)
		var m map[string]json.RawMessage
		if err := json.Unmarshal(result, &m); err != nil {
			t.Fatalf("args %v: unmarshal: %v", tc.args, err)
		}
		raw, ok := m[tc.key]
		if !ok {
			t.Errorf("args %v: key %q missing from result %s", tc.args, tc.key, result)
			continue
		}
		var got string
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Errorf("args %v: unmarshal value: %v", tc.args, err)
			continue
		}
		if got != tc.wantVal {
			t.Errorf("args %v: key %q = %q, want %q", tc.args, tc.key, got, tc.wantVal)
		}
	}
}
