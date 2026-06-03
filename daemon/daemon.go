package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/mattdurham/grv/hooks"
	"github.com/mattdurham/grv/ops"
)

// Request is the wire format for incoming tool calls.
type Request struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

// Response is the wire format for tool results.
type Response struct {
	Result json.RawMessage `json:"result"`
	Error  *string         `json:"error"`
}

// Server is the grv daemon.
type Server struct {
	Dir      string
	SockPath string
	PIDPath  string
	LogPath  string

	lastActivity atomic.Int64
	dispatch     map[string]func(json.RawMessage) (json.RawMessage, error)
	hookRunner   hooks.RunnerInterface
	repoRoot     string
}

// NewServer creates a new daemon Server.
func NewServer(dir, sockPath, pidPath, logPath string) *Server {
	s := &Server{
		Dir:      dir,
		SockPath: sockPath,
		PIDPath:  pidPath,
		LogPath:  logPath,
	}

	// Detect git repository root (best-effort; empty string if not a git repo)
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		s.repoRoot = strings.TrimSpace(string(out))
		ops.SetDefaultRepoRoot(s.repoRoot)
	}

	// Load hook config and create runner (optional — no error if config absent)
	if configs, checksConfig, err := hooks.LoadConfig(dir); err == nil {
		if len(configs) > 0 {
			s.hookRunner = hooks.New(configs, s.repoRoot)
			ops.SetDefaultHookRunner(s.hookRunner)
			go s.hookRunner.Warmup(dir)
		}
		ops.SetDefaultChecksConfig(checksConfig)
	}

	s.dispatch = s.buildDispatch()
	return s
}

// makeHandler creates a dispatch-table entry by unmarshalling raw JSON into T
// and calling the typed handler function.
func makeHandler[T any](fn func(T) (json.RawMessage, error)) func(json.RawMessage) (json.RawMessage, error) {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args T
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, fmt.Errorf("unmarshal args: %w", err)
			}
		}
		return fn(args)
	}
}

func (s *Server) buildDispatch() map[string]func(json.RawMessage) (json.RawMessage, error) {
	return map[string]func(json.RawMessage) (json.RawMessage, error){
		"ast_list":           makeHandler(ops.HandleASTList),
		"ast_query":          makeHandler(ops.HandleASTQuery),
		"ast_query_many":     makeHandler(ops.HandleASTQueryMany),
		"ast_meta":           makeHandler(ops.HandleASTMeta),
		"ast_insert":         makeHandler(ops.HandleASTInsert),
		"ast_insert_many":    makeHandler(ops.HandleASTInsertMany),
		"ast_replace":        makeHandler(ops.HandleASTReplace),
		"ast_replace_many":   makeHandler(ops.HandleASTReplaceMany),
		"ast_delete":         makeHandler(ops.HandleASTDelete),
		"ast_delete_many":    makeHandler(ops.HandleASTDeleteMany),
		"ast_add_import":     makeHandler(ops.HandleAddImport),
		"ast_delete_import":  makeHandler(ops.HandleDeleteImport),
		"ast_list_imports":   makeHandler(ops.HandleListImports),
		"gomod_read":         makeHandler(ops.HandleGoModRead),
		"gomod_require":      makeHandler(ops.HandleGoModRequire),
		"gomod_drop_require": makeHandler(ops.HandleGoModDropRequire),
		"gomod_replace":      makeHandler(ops.HandleGoModReplace),
		"gomod_drop_replace": makeHandler(ops.HandleGoModDropReplace),
		"ast_rename":         makeHandler(ops.HandleASTRename),
		"ast_node_at":        makeHandler(ops.HandleASTNodeAt),
		"ast_find_symbols":   makeHandler(ops.HandleASTFindSymbols),
		"ast_find":           makeHandler(ops.HandleASTFind),
		"ast_find_refs":      makeHandler(ops.HandleASTFindRefs),
		"ast_find_def":       makeHandler(ops.HandleASTFindDef),
		"ast_find_impls":     makeHandler(ops.HandleASTFindImpls),
		"ast_place":          makeHandler(ops.HandleASTPlace),
		"file_read":          makeHandler(ops.HandleFileRead),
		"file_write":         makeHandler(ops.HandleFileWrite),
		"ast_directory":      makeHandler(ops.HandleASTDirectory),
		"ast_check":          makeHandler(ops.HandleASTCheck),
	}
}

// Run starts the daemon, blocks until shutdown.
func (s *Server) Run() error {
	if s.PIDPath != "" {
		os.WriteFile(s.PIDPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
	}

	ln, err := net.Listen("unix", s.SockPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.SockPath, err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		ln.Close()
		os.Remove(s.SockPath)
		if s.PIDPath != "" {
			os.Remove(s.PIDPath)
		}
		os.Exit(0)
	}()

	s.touchActivity()
	go s.idleWatcher(ln)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return nil // listener closed
		}
		s.touchActivity()
		go s.handleConn(conn)
	}
}

func (s *Server) touchActivity() {
	s.lastActivity.Store(time.Now().UnixNano())
}

func (s *Server) idleWatcher(ln net.Listener) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if time.Since(time.Unix(0, s.lastActivity.Load())) > time.Hour {
			ln.Close()
			os.Remove(s.SockPath)
			if s.PIDPath != "" {
				os.Remove(s.PIDPath)
			}
			os.Exit(0)
		}
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var req Request
		if err := dec.Decode(&req); err != nil {
			return
		}
		s.touchActivity()
		enc.Encode(s.dispatchRequest(req))
		s.touchActivity()
	}
}
func (s *Server) dispatchRequest(req Request) Response {
	handler, ok := s.dispatch[req.Tool]
	if !ok {
		msg := fmt.Sprintf("unknown tool: %q", req.Tool)
		return Response{Error: &msg}
	}
	resolvedArgs, resolveErr := s.resolveNamespace(req.Tool, req.Args)
	if resolveErr != nil {
		msg := resolveErr.Error()
		return Response{Error: &msg}
	}
	req.Args = resolvedArgs
	result, err := handler(req.Args)
	if err != nil {
		msg := err.Error()
		return Response{Error: &msg}
	}
	if s.hookRunner != nil && isWriteTool(req.Tool) {
		if absFile := extractFileArg(req.Args); absFile != "" {
			s.hookRunner.Invalidate(absFile)
		}
	}
	return Response{Result: result}
}

// writeTools is the set of tools that modify file content.
var writeTools = map[string]bool{
	"ast_insert":       true,
	"ast_insert_many":  true,
	"ast_replace":      true,
	"ast_replace_many": true,
	"ast_delete":       true,
	"ast_delete_many":  true,
	"ast_rename":       true,
	"file_write":       true,
}

func isWriteTool(tool string) bool {
	return writeTools[tool]
}

// extractFileArg extracts the "file" field from a JSON args object.
// Returns empty string if not present.
func extractFileArg(raw json.RawMessage) string {
	if len(raw) == 0 || raw[0] != '{' {
		return ""
	}
	var m struct {
		File string `json:"file"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	return m.File
}
