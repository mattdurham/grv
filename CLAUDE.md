# grv — Invariants

1. grv never returns raw Go source text in any response field. All code representations are AST node trees (JSON objects with a `"kind"` discriminator). The `"source"` field must not appear in any response struct.

2. The AST node tree is the sole bidirectional representation: `ast_query` returns node trees; `ast_insert` and `ast_replace` accept node trees. There is no source-text shortcut on either side.

3. Every write tool (`ast_insert`, `ast_replace`, `ast_delete`, `ast_rename`, `file_write`) must parse the target file fresh on each call — no in-memory AST cache.

4. All file writes are atomic: write to a temp file in the same directory, then `os.Rename`. If `go/format` fails, the original file is untouched.

5. Readonly detection is enforced at the ops layer before any write. A file is readonly if it is under `vendor/`, `GOROOT`, the module cache, or has filesystem read-only permission.
