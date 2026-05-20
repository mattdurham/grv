# Implementation Plan: grv — Module Rename + Pure JSON Daemon

## Overview

Rename the Go module from `github.com/lthiery/goast` to `github.com/mattdurham/grv`, strip out all MCP code, and replace the stdio MCP transport with a Unix-socket JSON daemon and CLI. All 25 ops/ handler signatures change from `(ctx, req, args) (*mcp.CallToolResult, error)` to `(args T) (json.RawMessage, error)`. A new `daemon/` package serves requests over a per-CWD Unix socket with a 1-hour idle timeout. A new `cmd/` package handles start/stop/status and client invocation. main.go becomes a CLI dispatcher.

## Files to Create

1. `Makefile` — build, install, test, test-race, test-cover, clean targets
2. `daemon/daemon.go` — Unix socket server, dispatch table, idle timeout, SIGTERM handler
3. `daemon/daemon_test.go` — integration test: start daemon, send request, get response
4. `cmd/paths.go` — GRVDir(), SockPath(), PIDPath(), LogPath(), HashDir()
5. `cmd/start.go` — StartDaemon(dir): check-running, re-exec daemon subprocess, wait for socket
6. `cmd/stop.go` — StopDaemon(dir): read PID, kill, cleanup
7. `cmd/status.go` — Status(): list all .pid files in ~/.grv/, show running/dead
8. `cmd/client.go` — SendRequest(sockPath, toolName, argsJSON): connect, send, read, print, exit
9. `cmd/cmd_test.go` — tests for cmd/ functions against a real daemon

## Files to Delete

1. `server.go` — MCP tool registration, replaced by daemon dispatch table

## Files to Modify

1. `go.mod` — rename module path, drop mcp-go dependency
2. `main.go` — complete rewrite: CLI dispatcher (daemon/start/stop/status/tool/--json)
3. `ops/query.go` — remove ctx/req params from all handlers; replace toolError/navError helpers
4. `ops/insert.go` — remove ctx/req params from HandleASTInsert
5. `ops/replace.go` — remove ctx/req params from HandleASTReplace
6. `ops/delete.go` — remove ctx/req params from HandleASTDelete
7. `ops/imports.go` — remove ctx/req params from HandleAddImport, HandleDeleteImport, HandleListImports
8. `ops/gomod.go` — remove ctx/req params from all 5 gomod handlers
9. `ops/rename.go` — remove ctx/req params from HandleASTRename
10. `ops/lsp.go` — remove ctx/req params from HandleASTNodeAt, HandleASTFindSymbols, HandleASTFind
11. `ops/types.go` — remove ctx/req params from HandleASTFindRefs, HandleASTFindDef, HandleASTFindImpls
12. `ops/file.go` — remove ctx/req params from HandleFileRead, HandleFileWrite
13. `ops/directory.go` — remove ctx/req params from HandleASTDirectory
14. `ops/ops_test.go` — remove ctx/emptyReq, update all call sites
15. `ops/rename_test.go` — update call sites
16. `ops/lsp_test.go` — update call sites
17. `ops/types_test.go` — update call sites
18. `ops/file_test.go` — update call sites
19. `ops/directory_test.go` — update call sites
20. All `*.go` files — replace import path `github.com/lthiery/goast` with `github.com/mattdurham/grv`

## Implementation Steps

### Phase 0: Inventory (before writing any code)

**Step 0.1: Count all handler functions**
- [ ] Run `grep -rn "func Handle" ops/` to list all 25 handlers and confirm the full list
- [ ] Expected: HandleASTList, HandleASTQuery, HandleASTQueryMany, HandleASTMeta, HandleASTInsert, HandleASTReplace, HandleASTDelete, HandleAddImport, HandleDeleteImport, HandleListImports, HandleGoModRead, HandleGoModRequire, HandleGoModDropRequire, HandleGoModReplace, HandleGoModDropReplace, HandleASTRename, HandleASTNodeAt, HandleASTFindSymbols, HandleASTFind, HandleASTFindRefs, HandleASTFindDef, HandleASTFindImpls, HandleFileRead, HandleFileWrite, HandleASTDirectory

**Step 0.2: Catalog all test call sites**
- [ ] Run `grep -rn "emptyReq\|ctx, emptyReq" ops/*_test.go` to find all test call sites to update
- [ ] Run `grep -rn "type.*Args struct" ops/` to confirm all args type names for dispatch table

---

### Phase 1: Module Rename (Task 1)

**Step 1.1: Rename module path in go.mod**
- [ ] `go mod edit -module github.com/mattdurham/grv`
- [ ] Verify go.mod now reads `module github.com/mattdurham/grv`

**Step 1.2: Update all import paths in Go source files**
- [ ] `find . -name "*.go" | xargs sed -i 's|github.com/lthiery/goast|github.com/mattdurham/grv|g'`
- [ ] Verify with `grep -r "lthiery/goast" . --include="*.go"` — must return empty

**Step 1.3: Update log prefix strings**
- [ ] Replace `"goast: "` log prefix with `"grv: "` in main.go

**Step 1.4: Build verification**
- [ ] `go build ./...` — must pass before proceeding

---

### Phase 2: Makefile (Task 2, no deps)

**Step 2.1: Create Makefile**
- [ ] Create `Makefile` at repo root with these targets:

```makefile
.PHONY: install build test test-race test-cover clean

install: build
	install -m 755 grv /usr/local/bin/grv

build:
	go build -o grv .

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -f grv coverage.out
```

---

### Phase 3: Rewrite ops/ Handler Signatures (Task 3, blocked by Task 1)

#### New handler signature contract

Every handler in ops/ changes from:
```go
func HandleX(ctx context.Context, req mcp.CallToolRequest, args XArgs) (*mcp.CallToolResult, error)
```
to:
```go
func HandleX(args XArgs) (json.RawMessage, error)
```

#### New error helpers (in ops/query.go, shared within package)

Remove `toolError()` and `navError()`. Replace with:
```go
func errResult(msg string) (json.RawMessage, error) {
    return nil, fmt.Errorf("%s", msg)
}

func navErrResult(err *selector.NavigateError) (json.RawMessage, error) {
    resp := ErrorResponse{
        Error:     err.Error(),
        AtStep:    err.AtStep,
        Step:      &err.Step,
        Available: err.Available,
    }
    b, _ := json.Marshal(resp)
    return nil, fmt.Errorf("%s", string(b))
}

func okResult(v any) (json.RawMessage, error) {
    b, err := json.Marshal(v)
    if err != nil {
        return nil, fmt.Errorf("marshal result: %w", err)
    }
    return json.RawMessage(b), nil
}
```

The daemon wire protocol wraps errors in `{"result":null,"error":"..."}` — handlers return `(nil, error)` and the daemon serializes it.

**Step 3.1: Update ops/query.go**
- [ ] Remove `context` and `mcp` imports
- [ ] Replace `toolError()` with `errResult()`, `navError()` with `navErrResult()`
- [ ] Add `okResult()` helper
- [ ] Change HandleASTList, HandleASTQuery, HandleASTQueryMany, HandleASTMeta signatures
- [ ] Replace `mcp.NewToolResultText(string(b))` → `return okResult(result)` throughout
- [ ] Replace `mcp.NewToolResultError(...)` → `return errResult(...)` / `return navErrResult(...)`

**Step 3.2: Update ops/insert.go**
- [ ] Remove ctx/req params from HandleASTInsert
- [ ] Remove `mcp` import
- [ ] Update error returns: `toolError(msg)` → `errResult(msg)`, `navError(e)` → `navErrResult(e)`

**Step 3.3: Update ops/replace.go**
- [ ] Remove ctx/req params from HandleASTReplace
- [ ] Remove `mcp` import
- [ ] Update error returns

**Step 3.4: Update ops/delete.go**
- [ ] Remove ctx/req params from HandleASTDelete
- [ ] Remove `mcp` import
- [ ] Update error returns

**Step 3.5: Update ops/imports.go**
- [ ] Remove ctx/req params from HandleAddImport, HandleDeleteImport, HandleListImports
- [ ] Remove `mcp` import
- [ ] Update error returns

**Step 3.6: Update ops/gomod.go**
- [ ] Remove ctx/req params from HandleGoModRead, HandleGoModRequire, HandleGoModDropRequire, HandleGoModReplace, HandleGoModDropReplace
- [ ] Remove `mcp` import
- [ ] Update error returns

**Step 3.7: Update ops/rename.go**
- [ ] Remove ctx/req params from HandleASTRename
- [ ] Remove `context` and `mcp` imports
- [ ] Update error returns

**Step 3.8: Update ops/lsp.go**
- [ ] Remove ctx/req params from HandleASTNodeAt, HandleASTFindSymbols, HandleASTFind
- [ ] Remove `context` and `mcp` imports
- [ ] Update error returns

**Step 3.9: Update ops/types.go**
- [ ] Remove ctx/req params from HandleASTFindRefs, HandleASTFindDef, HandleASTFindImpls
- [ ] Remove `context` and `mcp` imports
- [ ] Update error returns

**Step 3.10: Update ops/file.go**
- [ ] Remove ctx/req params from HandleFileRead, HandleFileWrite
- [ ] Remove `context` and `mcp` imports
- [ ] Update error returns

**Step 3.11: Update ops/directory.go**
- [ ] Remove ctx/req params from HandleASTDirectory
- [ ] Remove `context` and `mcp` imports
- [ ] Update error returns

**Step 3.12: Update all ops/ test files**

The test helper `resultText()` in ops_test.go must change from:
```go
func resultText(t *testing.T, result *mcp.CallToolResult) string
```
to:
```go
func resultText(t *testing.T, result json.RawMessage, err error) string {
    t.Helper()
    if err != nil {
        t.Fatalf("tool returned error: %v", err)
    }
    return string(result)
}
```

- [ ] `ops/ops_test.go`: remove `ctx` var, remove `emptyReq` var, remove `mcp` import, update `resultText()`, update all handler calls (drop first two args), update `resultText(t, result)` to `resultText(t, result, err)`
- [ ] `ops/rename_test.go`: update all handler call sites
- [ ] `ops/lsp_test.go`: update all handler call sites
- [ ] `ops/types_test.go`: update all handler call sites
- [ ] `ops/file_test.go`: update all handler call sites
- [ ] `ops/directory_test.go`: update all handler call sites
- [ ] `ops/readonly_test.go`: check if it calls handlers; update if so

**Step 3.13: Build and test ops/**
- [ ] `go build ./ops/...` — must pass
- [ ] `go test ./ops/...` — must pass

---

### Phase 4: daemon/ Package (Task 4, blocked by Task 3)

**Step 4.1: Verify exported args types**

Before writing daemon.go, confirm all 25 args types are exported. Run:
```
grep -rn "type.*Args struct" ops/
```
Expected types: ASTListArgs, ASTQueryArgs, ASTQueryManyArgs, ASTMetaArgs, ASTInsertArgs, ASTReplaceArgs, ASTDeleteArgs, AddImportArgs, DeleteImportArgs, ListImportsArgs, GoModReadArgs, GoModRequireArgs, GoModDropRequireArgs, GoModReplaceArgs, GoModDropReplaceArgs, ASTRenameArgs, ASTNodeAtArgs, ASTFindSymbolsArgs, ASTFindArgs, ASTFindRefsArgs, ASTFindDefArgs, ASTFindImplsArgs, FileReadArgs, FileWriteArgs, ASTDirectoryArgs.

**Step 4.2: Create daemon/daemon.go**

Key types and functions:

```go
package daemon

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
}

func NewServer(dir, sockPath, pidPath, logPath string) *Server
func (s *Server) Run() error          // blocks; serves Unix socket
func (s *Server) touchActivity()
func (s *Server) idleWatcher(ln net.Listener)
func (s *Server) handleConn(conn net.Conn)
func (s *Server) dispatchRequest(req Request) Response
func (s *Server) buildDispatch() map[string]func(json.RawMessage) (json.RawMessage, error)

// Generic adapter: unmarshal raw JSON into typed T, call handler
func makeHandler[T any](fn func(T) (json.RawMessage, error)) func(json.RawMessage) (json.RawMessage, error)
```

**Dispatch table (all 25 entries):**
```go
"ast_list":           makeHandler(ops.HandleASTList),
"ast_query":          makeHandler(ops.HandleASTQuery),
"ast_query_many":     makeHandler(ops.HandleASTQueryMany),
"ast_meta":           makeHandler(ops.HandleASTMeta),
"ast_insert":         makeHandler(ops.HandleASTInsert),
"ast_replace":        makeHandler(ops.HandleASTReplace),
"ast_delete":         makeHandler(ops.HandleASTDelete),
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
"file_read":          makeHandler(ops.HandleFileRead),
"file_write":         makeHandler(ops.HandleFileWrite),
"ast_directory":      makeHandler(ops.HandleASTDirectory),
```

Note: `makeHandler` works directly because Go's generic type inference deduces T from the function argument type. `ops.HandleASTList` has type `func(ops.ASTListArgs) (json.RawMessage, error)` so T is inferred as `ops.ASTListArgs`.

**Run() implementation outline:**
```go
func (s *Server) Run() error {
    // Write PID file
    os.WriteFile(s.PIDPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)

    // Listen on Unix socket
    ln, err := net.Listen("unix", s.SockPath)

    // SIGTERM/SIGINT handler — cleanup + exit
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
    go func() { <-sig; ln.Close(); os.Remove(s.SockPath); os.Remove(s.PIDPath); os.Exit(0) }()

    // Idle watcher goroutine
    s.touchActivity()
    go s.idleWatcher(ln)

    // Accept loop
    for {
        conn, err := ln.Accept()
        if err != nil { return nil }  // listener closed
        s.touchActivity()
        go s.handleConn(conn)
    }
}
```

**idleWatcher outline:**
```go
func (s *Server) idleWatcher(ln net.Listener) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for range ticker.C {
        if time.Since(time.Unix(0, s.lastActivity.Load())) > time.Hour {
            ln.Close(); os.Remove(s.SockPath); os.Remove(s.PIDPath); os.Exit(0)
        }
    }
}
```

**handleConn outline:**
```go
func (s *Server) handleConn(conn net.Conn) {
    defer conn.Close()
    dec := json.NewDecoder(conn)
    enc := json.NewEncoder(conn)
    for {
        var req Request
        if err := dec.Decode(&req); err != nil { return }
        s.touchActivity()
        enc.Encode(s.dispatchRequest(req))
        s.touchActivity()
    }
}
```

**dispatchRequest outline:**
```go
func (s *Server) dispatchRequest(req Request) Response {
    handler, ok := s.dispatch[req.Tool]
    if !ok {
        msg := fmt.Sprintf("unknown tool: %q", req.Tool)
        return Response{Error: &msg}
    }
    result, err := handler(req.Args)
    if err != nil {
        msg := err.Error()
        return Response{Error: &msg}
    }
    return Response{Result: result}
}
```

**Step 4.3: Write daemon/daemon_test.go**

Test cases:
- [ ] TestDaemonHappyPath: start daemon in goroutine, wait for socket, send `{"tool":"ast_list","args":{"file":"<abs path to testdata/simple.go>"}}`, assert result is non-null JSON array
- [ ] TestDaemonKeepAlive: send two requests on same connection, assert both succeed
- [ ] TestDaemonUnknownTool: send unknown tool name, assert response has non-nil error field
- [ ] TestDaemonConcurrent: two goroutines each send a request, assert both get valid responses

For daemon_test.go, start the Server directly (not via subprocess):
```go
func startTestDaemon(t *testing.T) (sockPath string) {
    t.Helper()
    dir := t.TempDir()
    sockPath = filepath.Join(dir, "test.sock")
    pidPath  := filepath.Join(dir, "test.pid")
    s := daemon.NewServer(dir, sockPath, pidPath, "")
    go s.Run()
    // Wait for socket
    deadline := time.Now().Add(2 * time.Second)
    for time.Now().Before(deadline) {
        if _, err := os.Stat(sockPath); err == nil { return }
        time.Sleep(10 * time.Millisecond)
    }
    t.Fatal("daemon did not start")
    return
}
```

**Step 4.4: Build and test daemon/**
- [ ] `go build ./daemon/...` — must pass
- [ ] `go test ./daemon/...` — must pass

---

### Phase 5: cmd/ Package (Task 5, blocked by Task 4)

**Step 5.1: Create cmd/paths.go**

```go
package cmd

import (
    "crypto/sha256"
    "fmt"
    "os"
    "path/filepath"
)

// GRVDir returns ~/.grv, creating it if needed.
func GRVDir() (string, error) {
    home, err := os.UserHomeDir()
    if err != nil { return "", err }
    d := filepath.Join(home, ".grv")
    return d, os.MkdirAll(d, 0755)
}

// HashDir returns an 8-character hex string derived from the absolute directory path.
func HashDir(dir string) string {
    h := sha256.Sum256([]byte(dir))
    return fmt.Sprintf("%x", h[:4])
}

func SockPath(grvDir, hash string) string { return filepath.Join(grvDir, hash+".sock") }
func PIDPath(grvDir, hash string) string  { return filepath.Join(grvDir, hash+".pid") }
func LogPath(grvDir, hash string) string  { return filepath.Join(grvDir, hash+".log") }
```

**Step 5.2: Create cmd/start.go**

Key logic:
- `IsRunning(sockPath, pidPath string) bool` — read pid file, `proc.Signal(syscall.Signal(0)) == nil`
- `StartDaemon(dir string) error`:
  1. `GRVDir()` + `HashDir(dir)` → paths
  2. `IsRunning()` → return nil if already alive
  3. `os.Remove(sockPath)` + `os.Remove(pidPath)` — clean stale files
  4. `os.Executable()` → exe path
  5. `os.OpenFile(logPath, O_CREATE|O_APPEND|O_WRONLY, 0644)`
  6. `exec.Command(exe, "daemon", "--socket", sockPath, "--pid", pidPath, "--log", logPath, "--dir", dir)`
  7. `cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}`
  8. `cmd.Stdout = logF; cmd.Stderr = logF`
  9. `cmd.Start()`
  10. Poll for sockPath up to 2s with 50ms sleep intervals

**Step 5.3: Create cmd/stop.go**

`StopDaemon(dir string) error`:
1. Compute paths
2. Read pidPath → parse int PID
3. `os.FindProcess(pid)` → `proc.Signal(syscall.SIGTERM)`
4. `os.Remove(pidPath)` + `os.Remove(sockPath)` regardless

**Step 5.4: Create cmd/status.go**

`ListDaemons() ([]DaemonStatus, error)`:
- `os.ReadDir(grvDir)` → iterate `.pid` files
- For each: read PID, signal 0 to test liveness
- Return `[]DaemonStatus{Hash, PIDPath, SockPath, Alive, PID}`

`PrintStatus() error`:
- Print "no grv daemons running" if empty
- Otherwise print one line per daemon: `hash=X  running (pid N)  sock=/path`

**Step 5.5: Create cmd/client.go**

```go
// Request / Response types (wire format, same as daemon package)
type Request struct {
    Tool string          `json:"tool"`
    Args json.RawMessage `json:"args"`
}
type Response struct {
    Result json.RawMessage `json:"result"`
    Error  *string         `json:"error"`
}

func SendRequest(sockPath, toolName string, argsJSON json.RawMessage) (json.RawMessage, error) {
    conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
    defer conn.Close()
    json.NewEncoder(conn).Encode(Request{Tool: toolName, Args: argsJSON})
    var resp Response
    json.NewDecoder(conn).Decode(&resp)
    if resp.Error != nil { return nil, fmt.Errorf("%s", *resp.Error) }
    return resp.Result, nil
}
```

**Step 5.6: Create cmd/cmd_test.go**

Test cases:
- [ ] TestHashDir: same input → same 8-char hex, different inputs → different outputs
- [ ] TestGRVDir: creates directory, returns path ending in `.grv`
- [ ] TestPathFunctions: SockPath/PIDPath/LogPath return expected extensions
- [ ] TestIsRunningNoFile: returns false when pid file absent
- [ ] TestIsRunningDeadPID: returns false for PID 99999999 (or known-dead process)
- [ ] TestListDaemonsEmpty: returns empty slice when grvDir has no .pid files

---

### Phase 6: main.go Rewrite (Task 6, blocked by Task 5)

**Step 6.1: Delete server.go**
- [ ] Remove `server.go` — the MCP tool registration file is entirely replaced by the daemon dispatch table

**Step 6.2: Rewrite main.go**

Top-level switch:
```go
switch os.Args[1] {
case "daemon": runDaemon(os.Args[2:])         // internal: called by StartDaemon
case "start":  cmd.StartDaemon(dirArg())
case "stop":   cmd.StopDaemon(dirArg())
case "status": cmd.PrintStatus()
case "--json": runJSONMode()                   // read JSON from stdin
default:       runToolMode(os.Args[1:])        // grv TOOL --flag val
}
```

**runDaemon(args []string):** Parse `--socket`, `--pid`, `--log`, `--dir` flags; call `daemon.NewServer(...).Run()`

**runJSONMode():** `io.ReadAll(os.Stdin)` → parse `{"tool":"...","args":{...}}` → `sendAndPrint()`

**runToolMode(args []string):** `toolName := args[0]`; `argsJSON := parseToolFlags(toolName, args[1:])` → `sendAndPrint()`

**sendAndPrint(toolName string, argsJSON json.RawMessage):**
1. `dir := getCWD()`
2. `cmd.StartDaemon(dir)` — auto-start
3. Compute sockPath via GRVDir + HashDir
4. `cmd.SendRequest(sockPath, toolName, argsJSON)`
5. Print result JSON to stdout (or error response JSON to stdout with exit 1)

**parseToolFlags(toolName string, args []string) json.RawMessage:**
- Iterate args as `--key value` pairs
- Convert `--` prefix + replace `-` with `_` in key
- If value starts with `[` or `{` and is valid JSON: insert as raw JSON
- Otherwise: `json.Marshal(val)` (string)
- Boolean flag handling: if next arg starts with `--` or is absent, insert `true`
- Return `json.Marshal(m)`

**dirArg() string:** Returns `os.Args[2]` if len > 2, else `getCWD()`

**Step 6.3: Build verification**
- [ ] `go build ./...` — must pass

---

### Phase 7: Drop mcp-go Dependency (Task 7, blocked by Task 6)

**Step 7.1: Drop the dependency**
- [ ] `go mod edit -droprequire github.com/mark3labs/mcp-go`
- [ ] `go build ./...` — confirm no remaining mcp-go references (should be zero by now)
- [ ] `go mod tidy` — removes all transitive mcp-go deps

**Step 7.2: Verify final go.mod**

Expected remaining deps after tidy:
- `github.com/pmezard/go-difflib v1.0.0`
- `golang.org/x/mod v0.36.0`
- `golang.org/x/tools v0.45.0`
- Possibly `golang.org/x/sync` and `golang.org/x/text` if still used transitively

**Step 7.3: Final build**
- [ ] `go build -o grv .` — produces working binary
- [ ] `grep -r "mcp-go\|mark3labs\|lthiery" . --include="*.go"` — must return empty

---

### Phase 8: Final Verification (Task 8)

**Step 8.1: Full test suite**
- [ ] `go test ./...` — all pass
- [ ] `go test -race ./...` — no data races
- [ ] `go test -cover ./...` — review coverage

**Step 8.2: Binary smoke test**
- [ ] `go build -o grv .`
- [ ] `./grv` — prints usage, exits 1
- [ ] `./grv status` — prints daemon status (empty or running)
- [ ] `./grv ast_list --file /home/matt/source/goast-worktrees/goast-mcp-server/testdata/simple.go` — auto-starts daemon, returns JSON array
- [ ] Second invocation is faster (daemon already running)
- [ ] `./grv stop` — daemon stops
- [ ] `echo '{"tool":"ast_list","args":{"file":"/home/matt/source/goast-worktrees/goast-mcp-server/testdata/simple.go"}}' | ./grv --json` — works

**Step 8.3: Make targets**
- [ ] `make build` — produces `./grv`
- [ ] `make test` — all pass
- [ ] `make test-race` — no races
- [ ] `make clean` — removes binary and coverage.out

**Step 8.4: Final hygiene**
- [ ] `go fmt ./...` — no diffs
- [ ] `go vet ./...` — no issues

---

## Edge Cases to Handle

### Edge Case 1: Daemon socket path collision
**Scenario:** Two concurrent `grv` invocations for the same CWD race to start the daemon
**Expected:** Only one daemon starts; second detects running daemon via IsRunning() and skips launch
**Implementation:** `IsRunning()` checks PID liveness (signal 0); `net.Listen("unix", sockPath)` fails if already bound — second safeguard; stale file cleanup only if IsRunning returns false

### Edge Case 2: Stale PID/socket files from SIGKILL
**Scenario:** Daemon killed with SIGKILL leaves .pid and .sock files
**Expected:** `StartDaemon` detects dead process, removes stale files, starts fresh
**Implementation:** `IsRunning()` returns false for dead PID; `StartDaemon` calls `os.Remove` on both files before launching subprocess

### Edge Case 3: Boolean flags in parseToolFlags
**Scenario:** `grv ast_replace --file foo.go --dry_run` (no value after --dry_run)
**Expected:** `{"file":"foo.go","dry_run":true}` in args JSON
**Implementation:** In parseToolFlags, check if `i+1 >= len(args)` or `args[i+1]` starts with `--`; if so, insert `json.RawMessage("true")` for the flag and do not advance i

### Edge Case 4: null args in JSON request
**Scenario:** `{"tool":"ast_list","args":null}` sent to daemon
**Expected:** Handler receives zero-value ASTListArgs (File=""), returns error "parse: ..."
**Implementation:** `json.Unmarshal(nil, &args)` is a no-op in Go — zero value returned; handler naturally returns error for empty File field

### Edge Case 5: Daemon subprocess path resolution
**Scenario:** `grv` is run as `./grv` during development (not in PATH)
**Expected:** `StartDaemon` finds the correct binary to re-exec
**Implementation:** Use `os.Executable()` (not `os.Args[0]`) — resolves symlinks, returns absolute path; this handles both installed and development-build invocations

### Edge Case 6: Empty --json stdin
**Scenario:** `echo "" | grv --json` or `grv --json` with no stdin
**Expected:** Error message to stderr, exit 1
**Implementation:** `json.Unmarshal` of empty/whitespace bytes returns error → `log.Fatalf`

---

## Risks / Concerns

### Risk 1: makeHandler generics require Go 1.18+
**Risk:** `func makeHandler[T any]` requires Go 1.18+
**Impact:** Build failure on older Go
**Mitigation:** go.mod specifies `go 1.25.5` — no issue. Confirmed safe.

### Risk 2: resultText() signature change touches many test call sites
**Risk:** Every ops/ test calls resultText(); changing signature is a large mechanical edit
**Impact:** All ops/ tests fail to compile until all sites are updated
**Mitigation:** Use `grep -n "resultText" ops/*_test.go` before starting to enumerate all ~30+ sites; update ops_test.go first, then update call sites in other files one by one; do `go build ./ops/...` after each file to verify

### Risk 3: Args struct names may not match dispatch table expectations
**Risk:** Dispatch table in daemon.go uses specific exported type names that may not match what ops/ actually defines
**Impact:** Compile error in daemon/daemon.go
**Mitigation:** Step 4.1 explicitly runs `grep -rn "type.*Args struct" ops/` before writing the dispatch table to get exact names

### Risk 4: mcp-go may be referenced in go.sum but not code after Phase 3-6
**Risk:** `go mod tidy` in Phase 7 should clean go.sum, but if a test file was missed it could fail
**Impact:** `go build ./...` fails after droprequire
**Mitigation:** Run `go build ./...` before `go mod tidy` — catches any remaining mcp references; fix them, then tidy

### Risk 5: ops/readonly.go or ops/readonly_test.go may import mcp
**Risk:** These files were not listed in the "files to modify" list but may reference mcp types
**Impact:** Compile error after dropping mcp-go
**Mitigation:** Run `grep -rn "mcp" ops/` before finalizing Phase 3 to catch any missed files

---

## Dependencies

### Internal
- `daemon/` imports `ops/` (all 25 handler functions and their args types)
- `main` imports `cmd/` and `daemon/`
- All ops/ files import `github.com/mattdurham/grv/{editor,kinds,selector,meta}`

### External (keep)
- `golang.org/x/tools v0.45.0` — astutil, go/packages
- `golang.org/x/mod v0.36.0` — modfile
- `github.com/pmezard/go-difflib v1.0.0` — unified diff

### External (drop)
- `github.com/mark3labs/mcp-go` and all its transitive deps (jsonschema-go, uuid, santhosh-tekuri/jsonschema, spf13/cast, yosida95/uritemplate)

---

## Complexity Analysis

No function introduced by this plan is expected to approach complexity 40:
- `daemon.buildDispatch()`: map literal, complexity ~1
- `main.parseToolFlags()`: one loop with ~4 branches, complexity ~6
- `daemon.handleConn()`: single decode/encode loop, complexity ~3
- `cmd.StartDaemon()`: sequential with error checks, complexity ~8
- `main.main()`: switch on os.Args[1], complexity ~7

Existing ops/ handler complexity is unchanged.

---

## Execution Order

```
Phase 0 (inventory) — first, informs all later phases
Phase 1 (module rename) ──────┐
Phase 2 (Makefile)            │ — no deps on each other
                              ↓
              Phase 3 (ops/ signatures — blocked by Phase 1)
                              ↓
              Phase 4 (daemon/ — blocked by Phase 3)
                              ↓
              Phase 5 (cmd/ — blocked by Phase 4)
                              ↓
              Phase 6 (main.go — blocked by Phase 5)
                              ↓
              Phase 7 (drop mcp-go — blocked by Phase 6)
                              ↓
              Phase 8 (final verification)
```

---

## Success Criteria

- [ ] `go.mod` module is `github.com/mattdurham/grv`
- [ ] No references to `github.com/lthiery/goast` in any source file
- [ ] No references to `github.com/mark3labs/mcp-go` in any source file
- [ ] `go build -o grv .` succeeds
- [ ] `go test ./...` all pass
- [ ] `go test -race ./...` no data races
- [ ] `./grv ast_list --file <abs path to testdata/simple.go>` returns valid JSON
- [ ] `./grv --json` with valid stdin works
- [ ] `./grv start && ./grv stop && ./grv status` works end-to-end
- [ ] `make install` places binary at `/usr/local/bin/grv`
- [ ] All 25 handler functions have signature `func HandleX(args XArgs) (json.RawMessage, error)`
- [ ] Daemon dispatch table has exactly 25 entries
- [ ] Daemon auto-exits after 1 hour idle (watcher logic verified in tests)

## Notes

- `testdata/typesdata/` and `testdata/simple.go` must not be modified — they are test fixtures
- `readonly.go` and `readonly_test.go`: run `grep -rn "mcp" ops/readonly*.go` to check before assuming they need no changes
- The daemon `daemon` subcommand is internal (called only by cmd.StartDaemon) — omit it from usage help
- Wire protocol uses `json.Encoder.Encode()` which appends `\n` — `json.Decoder.Decode()` on the other end handles this correctly; the protocol is symmetric
- `makeHandler` generic type inference works in Go 1.18+: `makeHandler(ops.HandleASTList)` infers T=ops.ASTListArgs from the function argument type automatically — no explicit type parameter needed
- On the `--json` path, the response printed to stdout is the raw `json.RawMessage` result (not wrapped in `{"result":...}`) — matches what a direct tool call would print; error case prints `{"result":null,"error":"..."}` and exits 1
