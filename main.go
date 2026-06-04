package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/mattdurham/grv/cmd"
	"github.com/mattdurham/grv/daemon"
	"github.com/mattdurham/grv/treeformat"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("grv: ")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "daemon":
		runDaemon(os.Args[2:])
	case "start":
		dir := dirArg()
		if err := cmd.StartDaemon(dir); err != nil {
			log.Fatalf("start: %v", err)
		}
	case "stop":
		dir := dirArg()
		if err := cmd.StopDaemon(dir); err != nil {
			log.Fatalf("stop: %v", err)
		}
	case "status":
		if err := cmd.PrintStatus(); err != nil {
			log.Fatalf("status: %v", err)
		}
	case "help":
		filter := ""
		if len(os.Args) > 2 {
			filter = os.Args[2]
		}
		cmd.PrintHelp(filter)
	case "example":
		filter := ""
		if len(os.Args) > 2 {
			filter = os.Args[2]
		}
		cmd.PrintExamples(filter)
	case "guide":
		topic := ""
		if len(os.Args) > 2 {
			topic = os.Args[2]
		}
		cmd.PrintGuide(topic)
	case "grammar":
		filter := ""
		if len(os.Args) > 2 {
			filter = os.Args[2]
		}
		cmd.PrintGrammar(filter)
	case "--json":
		runJSONMode()
	case "convert":
		dir := getCWD()
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		cmd.RunConvert(dir)
	default:
		runToolMode(os.Args[1:])
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: grv <tool> [--flag value ...] | grv --json | grv start|stop|status [dir]")
}

func dirArg() string {
	if len(os.Args) > 2 {
		return os.Args[2]
	}
	return getCWD()
}

func getCWD() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("getwd: %v", err)
	}
	return dir
}

func runDaemon(args []string) {
	var sockPath, pidPath, logPath, dir string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--socket":
			i++
			if i < len(args) {
				sockPath = args[i]
			}
		case "--pid":
			i++
			if i < len(args) {
				pidPath = args[i]
			}
		case "--log":
			i++
			if i < len(args) {
				logPath = args[i]
			}
		case "--dir":
			i++
			if i < len(args) {
				dir = args[i]
			}
		}
	}
	if sockPath == "" {
		log.Fatal("daemon: --socket is required")
	}
	if dir == "" {
		dir = getCWD()
	}
	if logPath != "" {
		logF, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			log.SetOutput(logF)
		}
	}
	s := daemon.NewServer(dir, sockPath, pidPath, logPath)
	if err := s.Run(); err != nil {
		log.Fatalf("daemon: %v", err)
	}
}

// runJSONMode reads a {tool, args} JSON request from stdin and runs it.
// Used for programmatic / MCP integration.
func runJSONMode() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("read stdin: %v", err)
	}
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		log.Fatal("empty input")
	}
	var req struct {
		Tool string          `json:"tool"`
		Args json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		log.Fatalf("parse request: %v", err)
	}
	sendAndPrint(req.Tool, req.Args)
}

func runToolMode(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}
	toolName := args[0]
	argsJSON := parseToolFlags(toolName, args[1:])
	sendAndPrint(toolName, argsJSON)
}

func sendAndPrint(toolName string, argsJSON json.RawMessage) {
	dir := getCWD()
	if err := cmd.StartDaemon(dir); err != nil {
		log.Fatalf("start daemon: %v", err)
	}
	grvDir, err := cmd.GRVDir()
	if err != nil {
		log.Fatalf("grv dir: %v", err)
	}
	sockPath := cmd.SockPath(grvDir)
	result, err := cmd.SendRequest(sockPath, toolName, argsJSON)
	if err != nil {
		errResp := map[string]any{"result": nil, "error": err.Error()}
		b, _ := json.Marshal(errResp) // error response struct: Marshal never fails
		fmt.Println(string(b))
		os.Exit(1)
	}
	// Always output tree notation.
	if out, err := treeformat.Marshal(result); err == nil {
		fmt.Println(string(out))
		return
	}
	fmt.Println(string(result))
}

// parseToolFlags encodes CLI flag values into a JSON args map.
// All node/path/ops values use tree notation — JSON is not accepted.
func parseToolFlags(toolName string, args []string) json.RawMessage {
	m := make(map[string]json.RawMessage)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			if _, exists := m["namespace"]; !exists {
				b, _ := json.Marshal(arg) // string arg: Marshal never fails
				m["namespace"] = b
			}
			continue
		}
		key := strings.TrimPrefix(arg, "--")
		var val string
		var hasEmbeddedVal bool
		if k, v, ok := strings.Cut(key, "="); ok {
			key, val, hasEmbeddedVal = k, v, true
		}
		key = strings.ReplaceAll(key, "-", "_")

		if hasEmbeddedVal {
			// --key=value form: val already set.
		} else if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			m[key] = json.RawMessage("true")
			continue
		} else {
			i++
			val = args[i]
		}

		switch key {
		case "node":
			// Tree notation node → JSON AST.
			if raw, err := treeformat.Unmarshal([]byte(val)); err == nil {
				m[key] = raw
			} else {
				log.Fatalf("--node: invalid tree notation: %v\nRun: grv guide notation", err)
			}
		case "path":
			// Tree path → JSON step array.
			if raw := parseTreePath(val); raw != nil {
				m[key] = raw
			} else {
				log.Fatalf("--path: invalid tree path %q\nExample: 'FuncDecl name=foo / BlockStmt'", val)
			}
		case "paths":
			// Multiple tree paths separated by --- → JSON array of step arrays.
			if raw := parseTreePaths(val); raw != nil {
				m[key] = raw
			} else {
				log.Fatalf("--paths: invalid tree paths\nSeparate paths with --- on its own line")
			}
		case "ops":
			// Operation list — format depends on tool.
			if raw, err := parseTreeOps(toolName, val); err == nil {
				m[key] = raw
			} else {
				log.Fatalf("--ops: %v\nRun: grv guide notation", err)
			}
		case "pattern":
			// Tree notation pattern → JSON node.
			if raw, err := treeformat.Unmarshal([]byte(val)); err == nil {
				m[key] = raw
			} else {
				log.Fatalf("--pattern: invalid tree notation: %v", err)
			}
		default:
			m[key] = encodeValue(val)
		}
	}
	result, _ := json.Marshal(m) // map of RawMessage: Marshal never fails
	return result
}

// parseTreePath converts "FuncDecl name=foo / BlockStmt" into a JSON step array.
func parseTreePath(val string) json.RawMessage {
	val = strings.TrimSpace(val)
	var parts []string
	if strings.Contains(val, " / ") {
		parts = strings.Split(val, " / ")
	} else if strings.Contains(val, "\n") {
		parts = strings.Split(val, "\n")
	} else {
		parts = []string{val}
	}
	steps := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) == 0 || !unicode.IsUpper(rune(p[0])) {
			return nil
		}
		step := map[string]any{}
		tokens := strings.Fields(p)
		step["kind"] = tokens[0]
		for _, kv := range tokens[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			var parsed any
			if err := json.Unmarshal([]byte(v), &parsed); err == nil {
				step[k] = parsed
			} else {
				step[k] = v
			}
		}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return nil
	}
	b, _ := json.Marshal(steps)
	return b
}

// parseTreePaths converts multiple tree paths separated by "---" into a JSON
// array of step arrays, for use with ast_query_many --paths.
//
// Input:
//
//	FuncDecl name=foo
//	---
//	FuncDecl name=bar / BlockStmt
func parseTreePaths(val string) json.RawMessage {
	blocks := splitBlocks(val)
	paths := make([]json.RawMessage, 0, len(blocks))
	for _, b := range blocks {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		raw := parseTreePath(b)
		if raw == nil {
			return nil
		}
		paths = append(paths, raw)
	}
	if len(paths) == 0 {
		return nil
	}
	b, _ := json.Marshal(paths)
	return b
}

// parseTreeOps converts a tree-notation ops string into a JSON ops array.
// The format depends on the tool:
//
// ast_patch:
//
//	set <field> <value>
//	delete <field>
//	delete <field> <index>
//	append <field>
//	  <tree-node...>
//	prepend <field>
//	  <tree-node...>
//	insert <field> <index>
//	  <tree-node...>
//
// ast_replace_many, ast_insert_many, ast_delete_many (--- separated blocks):
//
//	path FuncDecl name=foo
//	index -1               (insert_many only)
//	node
//	  FuncDecl name=bar
func parseTreeOps(toolName, val string) (json.RawMessage, error) {
	switch toolName {
	case "ast_patch":
		return parsePatchOps(val)
	case "ast_replace_many", "ast_insert_many", "ast_delete_many":
		return parseBatchOps(toolName, val)
	default:
		return nil, fmt.Errorf("--ops not supported for %s", toolName)
	}
}

// parsePatchOps parses ast_patch ops notation.
func parsePatchOps(val string) (json.RawMessage, error) {
	lines := strings.Split(strings.TrimSpace(val), "\n")
	var ops []map[string]any
	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}
		tokens := strings.Fields(line)
		if len(tokens) == 0 {
			i++
			continue
		}
		op := map[string]any{"op": tokens[0]}
		switch tokens[0] {
		case "set":
			if len(tokens) < 3 {
				return nil, fmt.Errorf("set requires: set <field> <value>")
			}
			op["field"] = tokens[1]
			rawVal := strings.Join(tokens[2:], " ")
			var v any
			if err := json.Unmarshal([]byte(rawVal), &v); err == nil {
				b, _ := json.Marshal(v)
				op["value"] = json.RawMessage(b)
			} else {
				b, _ := json.Marshal(rawVal)
				op["value"] = json.RawMessage(b)
			}
			i++
		case "delete":
			if len(tokens) < 2 {
				return nil, fmt.Errorf("delete requires: delete <field> [index]")
			}
			op["field"] = tokens[1]
			if len(tokens) >= 3 {
				if idx, err := strconv.Atoi(tokens[2]); err == nil {
					op["index"] = idx
				}
			}
			i++
		case "append", "prepend":
			if len(tokens) < 2 {
				return nil, fmt.Errorf("%s requires: %s <field>\\n  <tree-node>", tokens[0], tokens[0])
			}
			op["field"] = tokens[1]
			i++
			nodeLines, next := collectIndented(lines, i, 2)
			if len(nodeLines) == 0 {
				return nil, fmt.Errorf("%s: missing node (indent 2 spaces below op)", tokens[0])
			}
			raw, err := treeformat.Unmarshal([]byte(strings.Join(nodeLines, "\n")))
			if err != nil {
				return nil, fmt.Errorf("%s node: %v", tokens[0], err)
			}
			op["value"] = raw
			i = next
		case "insert":
			if len(tokens) < 3 {
				return nil, fmt.Errorf("insert requires: insert <field> <index>\\n  <tree-node>")
			}
			op["field"] = tokens[1]
			idx, err := strconv.Atoi(tokens[2])
			if err != nil {
				return nil, fmt.Errorf("insert: index must be an integer, got %q", tokens[2])
			}
			op["index"] = idx
			i++
			nodeLines, next := collectIndented(lines, i, 2)
			if len(nodeLines) == 0 {
				return nil, fmt.Errorf("insert: missing node (indent 2 spaces below op)")
			}
			raw, err := treeformat.Unmarshal([]byte(strings.Join(nodeLines, "\n")))
			if err != nil {
				return nil, fmt.Errorf("insert node: %v", err)
			}
			op["value"] = raw
			i = next
		default:
			return nil, fmt.Errorf("unknown op %q — valid: set, delete, append, prepend, insert", tokens[0])
		}
		ops = append(ops, op)
	}
	b, _ := json.Marshal(ops)
	return b, nil
}

// parseBatchOps parses replace_many / insert_many / delete_many ops notation.
// Blocks separated by "---".
func parseBatchOps(toolName, val string) (json.RawMessage, error) {
	blocks := splitBlocks(val)
	var ops []map[string]any
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		op := map[string]any{}
		lines := strings.Split(block, "\n")
		i := 0
		for i < len(lines) {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				i++
				continue
			}
			tokens := strings.Fields(trimmed)
			switch tokens[0] {
			case "path":
				pathStr := strings.TrimSpace(strings.TrimPrefix(trimmed, "path"))
				raw := parseTreePath(pathStr)
				if raw == nil {
					return nil, fmt.Errorf("invalid path %q in ops block", pathStr)
				}
				op["path"] = raw
				i++
			case "index":
				if len(tokens) < 2 {
					return nil, fmt.Errorf("index requires a value")
				}
				idx, err := strconv.Atoi(tokens[1])
				if err != nil {
					return nil, fmt.Errorf("index must be integer, got %q", tokens[1])
				}
				op["index"] = idx
				i++
			case "node":
				i++
				nodeLines, next := collectIndented(lines, i, 2)
				if len(nodeLines) == 0 {
					return nil, fmt.Errorf("node: missing tree node (indent 2 spaces)")
				}
				raw, err := treeformat.Unmarshal([]byte(strings.Join(nodeLines, "\n")))
				if err != nil {
					return nil, fmt.Errorf("node: %v", err)
				}
				op["node"] = raw
				i = next
			default:
				// Bare path (delete_many shorthand: just the path on its own line).
				if unicode.IsUpper(rune(trimmed[0])) {
					raw := parseTreePath(trimmed)
					if raw == nil {
						return nil, fmt.Errorf("invalid path %q", trimmed)
					}
					op["path"] = raw
				}
				i++
			}
		}
		if len(op) > 0 {
			ops = append(ops, op)
		}
	}
	b, _ := json.Marshal(ops)
	return b, nil
}

// splitBlocks splits a string on lines containing only "---".
func splitBlocks(val string) []string {
	var blocks []string
	var cur strings.Builder
	for _, line := range strings.Split(val, "\n") {
		if strings.TrimSpace(line) == "---" {
			blocks = append(blocks, cur.String())
			cur.Reset()
		} else {
			cur.WriteString(line)
			cur.WriteByte('\n')
		}
	}
	blocks = append(blocks, cur.String())
	return blocks
}

// collectIndented returns lines[start:] that have at least minIndent spaces,
// with the leading minIndent spaces stripped. Returns the lines and next index.
func collectIndented(lines []string, start, minIndent int) ([]string, int) {
	prefix := strings.Repeat(" ", minIndent)
	var result []string
	i := start
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}
		if !strings.HasPrefix(line, prefix) {
			break
		}
		result = append(result, line[minIndent:])
		i++
	}
	return result, i
}

func encodeValue(val string) json.RawMessage {
	b, _ := json.Marshal(val)
	return b
}
