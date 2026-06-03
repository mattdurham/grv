Task: Batch write ops + 3 new ast_check rules for grv

Feature 1 — Batch writes:
- ast_insert_many: array of {path, index, node} applied in one editor.Edit call
- ast_replace_many: array of {path, node} applied in one editor.Edit call  
- ast_delete_many: array of {path} applied in one editor.Edit call
- All atomic: if any op fails, whole batch rejected, file untouched
- enforcePostWrite runs once after all mutations
- Wire into daemon dispatch + cmd/help.go

Feature 2 — New rules in ops/checks.go builtinRules:
- type_assertion_not_checked: single-value t := i.(T) can panic
- mutex_not_embedded: sync.Mutex/RWMutex embedded (anonymous) in struct
- channel_size_not_one_or_zero: make(chan T, N) where N>1 with no comment

Spec-driven modules: ops/ has NOTE invariants on .go files
Key files: ops/insert.go, ops/replace.go, ops/delete.go, ops/checks.go, daemon/daemon.go, cmd/help.go
