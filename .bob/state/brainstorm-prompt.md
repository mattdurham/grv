# grv — Module Rename + Pure JSON Daemon (no MCP)

## What we're building
A standalone Go AST code intelligence daemon with a CLI. No MCP protocol, no
mark3labs/mcp-go dependency. Claude calls grv via bash.

## Task
1. Rename Go module: github.com/lthiery/goast → github.com/mattdurham/grv
2. Rename binary: goast → grv
3. Remove ALL MCP code (server.go rewrite, drop mcp-go dependency)
4. Replace with direct JSON-over-Unix-socket daemon + CLI
5. Makefile: install, test, test-race, build, clean

## Architecture

```
Claude (bash tool)
    │
    ▼  echo '{"tool":"ast_list","args":{"file":"foo.go"}}' | grv
grv CLI
    │  auto-start daemon for CWD if not running
    │  send JSON request over Unix socket
    │  print JSON response to stdout
    │  exit 0
    ▼
grv daemon (per-CWD, background)
    │  ~/.grv/<hash8(abs_cwd)>.sock
    │  1-hour idle timeout
    │  25 tool handlers (same logic, different transport)
    ▼
JSON response {"result": ..., "error": null}
```

## Wire protocol (simple JSON lines)
Request (one JSON object per line):
```json
{"tool": "ast_list", "args": {"file": "internal/lexer.go"}}
```
Response (one JSON object):
```json
{"result": [...], "error": null}
```
Error response:
```json
{"result": null, "error": "file not found"}
```

## CLI usage
```bash
# Direct invocation (grv auto-starts daemon, sends request, prints result, exits)
grv ast_list --file lexer.go
grv ast_query --file lexer.go --path '[{"kind":"FuncDecl","name":"advance"}]'
grv file_read --file config.yaml
grv ast_find --file lexer.go --pattern '{"kind":"IfStmt"}'

# Or JSON mode:
echo '{"tool":"ast_list","args":{"file":"lexer.go"}}' | grv --json
grv --json <<'EOF'
{"tool":"ast_rename","args":{"file":"lexer.go","path":[...],"to":"scan","dry_run":true}}
EOF

# Daemon management
grv start [dir]   # explicit daemon start
grv stop [dir]    # stop daemon
grv status        # list running daemons
```

## Tool handlers
Keep ALL 25 handler functions from ops/ package. Remove the mcp.CallToolRequest
parameter — replace with plain args structs. Handler signature becomes:
```go
func HandleASTList(args ASTListArgs) (json.RawMessage, error)
```

## Daemon internals
- net.Listener on ~/.grv/<hash8>.sock
- Each connection: read JSON line → dispatch to handler → write JSON response line → keep alive for next request on same connection
- lastActivity atomic.Int64; background goroutine exits after 1hr idle
- Graceful shutdown: SIGTERM handler removes socket + pid files
- PID file: ~/.grv/<hash8>.pid
- Log file: ~/.grv/<hash8>.log

## Daemon startup (grv start / auto-start)
```go
cmd := exec.Command(os.Args[0], "daemon", "--socket", sock, "--dir", dir)
cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
logF, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
cmd.Stdout, cmd.Stderr = logF, logF
cmd.Start()  // fire and forget
// wait up to 2s for socket to appear
```

## Makefile
```makefile
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

## help and grammar commands

These are pure local commands — no daemon needed, no socket connection.

### grv help [tool]
Lists all 25 tool names with their descriptions and argument schemas.

```bash
$ grv help
ast_list        List all top-level declarations in a Go file
                Args: file (string, required)

ast_query       Return the JSON node tree at a path
                Args: file (string, required), path ([]PathStep, required)

ast_rename      Rename an identifier (AST-only, single-file)
                Args: file, path, to, dry_run
...

$ grv help ast_rename
ast_rename — Rename an identifier at its declaration site

  Args:
    file     string   required   Path to Go source file
    path     []step   required   Selector path to declaration
    to       string   required   New identifier name
    dry_run  bool     optional   Return diff without writing (default: false)

  Notes: AST-only approximation. Renames all matching identifiers regardless
         of scope. Accurate for top-level declarations.
```

Implemented as: a static registry in `cmd/help.go` — each tool has a ToolInfo{Name, Description, Args[]ArgInfo, Notes string}.

### grv grammar [kind]
Lists the JSON node schema for Go AST node kinds — the grammar for constructing
and reading node trees.

```bash
$ grv grammar
Ident           {"kind":"Ident","name":"string"}
BasicLit        {"kind":"BasicLit","tok":"INT|FLOAT|IMAG|CHAR|STRING","value":"string"}
BinaryExpr      {"kind":"BinaryExpr","x":<Expr>,"op":"string","y":<Expr>}
...

$ grv grammar IfStmt
IfStmt — An if statement

  Schema:
    {"kind":     "IfStmt",
     "init":     <Stmt|null>,     // optional init statement: if x := f(); x > 0
     "cond":     <Expr>,          // condition expression (required)
     "body":     <BlockStmt>,     // then-branch body (required)
     "else":     <Stmt|null>}     // else branch: IfStmt or BlockStmt

  Example:
    // if x > 0 { return x }
    {"kind": "IfStmt",
     "cond": {"kind": "BinaryExpr",
               "x": {"kind": "Ident", "name": "x"},
               "op": ">",
               "y": {"kind": "BasicLit", "tok": "INT", "value": "0"}},
     "body": {"kind": "BlockStmt",
               "list": [{"kind": "ReturnStmt",
                          "results": [{"kind": "Ident", "name": "x"}]}]}}

  Notes: true/false/nil are Ident nodes, not BasicLit.
         init and else are omitted when null.
```

Implemented as: a static registry in `cmd/grammar.go` — each of the 50 kinds has a KindDoc{Schema, Example, Notes}. Generated from the kinds package structs + hand-written examples for the complex ones.

## Dependency removal
- Remove: github.com/mark3labs/mcp-go
- Keep: golang.org/x/tools, golang.org/x/mod, github.com/pmezard/go-difflib
- All ops/ handlers: remove context.Context and mcp.CallToolRequest params
