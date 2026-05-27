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
