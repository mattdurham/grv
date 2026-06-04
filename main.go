package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

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
	sendAndPrint(req.Tool, req.Args, "json")

}

func runToolMode(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}
	format, args := extractFormat(args)
	toolName := args[0]
	argsJSON := parseToolFlags(args[1:])
	sendAndPrint(toolName, argsJSON, format)

}
func sendAndPrint(toolName string, argsJSON json.RawMessage, format string) {
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
		errResp := map[string]interface {
		}{"result": nil, "error": err.Error()}
		b, _ := json.Marshal(errResp)
		fmt.Println(string(b))
		os.Exit(1)
	}
	if format == "tree" && isASTTool(toolName) {
		if out, err := treeformat.Marshal(result); err == nil {
			fmt.Println(string(out))
			return
		}
	}

	fmt.Println(string(result))
}

func parseToolFlags(args []string) json.RawMessage {
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
			// --key=value form: val already set, fall through to marshal below
		} else if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			// Boolean flag
			m[key] = json.RawMessage("true")
			continue
		} else {
			i++
			val = args[i]
		}

		// Check if val is raw JSON (object or array)
		if len(val) > 0 && (val[0] == '{' || val[0] == '[') {
			var raw json.RawMessage
			if json.Unmarshal([]byte(val), &raw) == nil {
				m[key] = raw
				continue
			}
		}
		// --node in tree notation: auto-detect and convert to JSON.
		if key == "node" && !json.Valid([]byte(val)) {
			if raw, err := treeformat.Unmarshal([]byte(val)); err == nil {
				m[key] = raw
				continue
			}
		}
		// --path / --paths in tree notation: "FuncDecl name=foo / BlockStmt" → JSON step array.
		if (key == "path" || key == "paths") && !json.Valid([]byte(val)) {
			if raw := parseTreePath(val); raw != nil {
				m[key] = raw
				continue
			}
		}
		// Otherwise treat as string.
		m[key] = encodeValue(val)

	}

	result, _ := json.Marshal(m) // map of RawMessage: Marshal never fails
	return result
}
// parseTreePath converts a tree-notation path like "FuncDecl name=foo / BlockStmt"
// into a JSON step array: [{"kind":"FuncDecl","name":"foo"},{"kind":"BlockStmt"}].
// Returns nil if the input doesn't look like tree-notation steps.
func parseTreePath(val string) json.RawMessage {
	// Split on " / " (slash separator) or newlines.
	val = strings.TrimSpace(val)
	var parts []string
	if strings.Contains(val, " / ") {
		parts = strings.Split(val, " / ")
	} else if strings.Contains(val, "\n") {
		parts = strings.Split(val, "\n")
	} else {
		parts = []string{val}
	}

	steps := make([]map[string]interface{}, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Each part is a tree node header: "KindName attr=val ..."
		// Must start with uppercase to be a valid kind name.
		if len(p) == 0 || p[0] < 'A' || p[0] > 'Z' {
			return nil
		}
		step := map[string]interface{}{}
		tokens := strings.Fields(p)
		step["kind"] = tokens[0]
		for _, kv := range tokens[1:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			// Parse the value: try JSON first (numbers, bools, quoted strings), else keep as string.
			var parsed interface{}
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
	b, err := json.Marshal(steps)
	if err != nil {
		return nil
	}
	return json.RawMessage(b)
}

func extractFormat(args []string) (string, []string) {
	format := "tree"
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--format=") {
			format = strings.TrimPrefix(arg, "--format=")
			continue
		}
		if arg == "--format" && i+1 < len(args) {
			i++
			format = args[i]
			continue
		}
		out = append(out, arg)
	}
	return format, out
}
func encodeValue(val string) json.RawMessage {
	b, _ := json.Marshal(val)
	return b
}

func isASTTool(name string) bool {
	switch name {
	case "ast_query", "ast_list", "ast_query_many", "ast_meta", "ast_directory", "ast_find", "ast_find_symbols", "ast_node_at", "ast_check", "ast_find_refs", "ast_find_def", "ast_find_impls", "ast_list_imports", "gomod_read", "ast_insert", "ast_replace", "ast_insert_many", "ast_replace_many", "ast_delete", "ast_delete_many", "ast_patch", "ast_rename":
		return true
	default:
		return false
	}
}
