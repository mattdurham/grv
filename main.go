package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattdurham/grv/cmd"
	"github.com/mattdurham/grv/daemon"
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

	sendAndPrint(req.Tool, req.Args)
}

func runToolMode(args []string) {
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}
	toolName := args[0]
	argsJSON := parseToolFlags(args[1:])
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
	hash := cmd.HashDir(dir)
	sockPath := cmd.SockPath(grvDir, hash)

	result, err := cmd.SendRequest(sockPath, toolName, argsJSON)
	if err != nil {
		errResp := map[string]interface{}{
			"result": nil,
			"error":  err.Error(),
		}
		b, _ := json.Marshal(errResp)
		fmt.Println(string(b))
		os.Exit(1)
	}
	fmt.Println(string(result))
}

func parseToolFlags(args []string) json.RawMessage {
	m := make(map[string]json.RawMessage)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		key := strings.TrimPrefix(arg, "--")
		key = strings.ReplaceAll(key, "-", "_")

		// Check if there's a value
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			// Boolean flag
			m[key] = json.RawMessage("true")
			continue
		}
		i++
		val := args[i]

		// Check if val is raw JSON (object or array)
		if len(val) > 0 && (val[0] == '{' || val[0] == '[') {
			var raw json.RawMessage
			if json.Unmarshal([]byte(val), &raw) == nil {
				m[key] = raw
				continue
			}
		}
		// Otherwise treat as string
		b, _ := json.Marshal(val)
		m[key] = b
	}

	result, _ := json.Marshal(m)
	return result
}

func abs(path string) string {
	a, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return a
}
