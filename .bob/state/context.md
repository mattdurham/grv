# Agent Context

## Role & Principles
Use grv tools for all Go source file reads and writes — grv file_read, grv file_write, grv ast_insert, grv ast_replace, grv ast_delete, grv ast_rename. Never use Read/Write/Edit tools or sed/awk on .go files.

> id: 100abe24-403f-44cc-9c59-70d705dc2894

# Behavioral Rule

**Perform complex transformations through abstraction layers rather than direct resource manipulation.**

Or more concisely:

**Use tool APIs over raw access when available.**

---

## Why This Captures It

The skill demonstrates systematically preferring:
- AST query/insert/replace APIs over text editing
- Implicit path resolution (injectDir middleware) over explicit file paths
- Tool-mediated operations over direct I/O

This pattern generalizes beyond code: it's about letting intermediate abstractions handle resource logistics so agents can focus on the actual problem (optimization logic). The proof of concept specifically validates that this approach scales to production outcomes, not just simpler tasks.

> id: ea804344-1d3b-4d3f-8352-dddb39d20269

# Behavioral Rule

**Escape special characters before parsing, not after — context-specific delimiters require pre-processing at the call site before generic parsers consume them.**

Or more concisely:

**Pre-escape context delimiters before generic parsing functions.**

The principle: Generic parsers (like `url.Parse`) have built-in interpretations of certain characters (like `#` for fragments). These interpretations happen during parsing and cannot be undone by post-hoc escaping. Characters that are semantically significant to the parser must be escaped *before* handing data to it, at the point where you still control the raw input.

> id: 221ff919-1019-4f12-87b4-3e8cd7776bbe

# Behavioral Rule

**Escape special characters before parsing, not after.**

Or more specifically: **Handle context-specific delimiters at the input layer before generic parsers process the data.**

---

## Explanation

This rule captures that certain characters have special meaning in specific contexts (here, `#` in URLs). Generic parsers like `url.Parse` treat these as structural delimiters, so they must be escaped *before* the parser sees them—not by relying on the parser's escape functions afterward. The parser has already made its interpretation by then.

> id: a18375ff-d18a-43dd-b347-81b5d9090645

# Rule

**Replace sparse hash structures with dense bit-vectors when the domain is small, bounded, and integer-indexed.**

Or more concisely:

**Bounded integer sets → bitsets; unbounded or sparse → hash maps.**

---

## Why this generalizes

The underlying principle is:
- **Hash maps** pay per-element overhead (hashing, bucket allocation, pointer chasing) — optimal for sparse or unbounded domains
- **Bitsets** pay fixed overhead per bit-range — optimal for dense, bounded domains where indices fit in a small integer space

The choice hinges on the **density-to-overhead ratio**, not the data structure's prestige. When N is small and elements cluster in [0, N), bitwise ops on a word array dominate.

**Corollary**: The "no index" sentinel (bool=false for nil) reflects that bitsets need an explicit "unconstrained" marker where hash maps use absence.

> id: ec233da4-b145-49e4-a9cf-b7fb86808067


## Relevant Techniques
# Skill: Pragmatic AST Manipulation Through Constraint-Aware Decomposition

**Core Pattern**: When working with AST modification tools that have structural limitations, break complex refactoring tasks into smaller, ordered operations that work *with* rather than *against* tool constraints.

**Key Techniques**:
- **Decompose over brute-force**: Replace entire nodes rather than attempting partial sub-node modifications; break large rewrites into sequential targeted operations (delete → insert → update)
- **Use intermediate staging**: Leverage temporary files or existing code patterns as AST templates to extract, verify, and modify incrementally rather than constructing complex JSON structures from scratch
- **Order changes strategically**: Execute modifications in dependency order (new blocks first, then signatures, avoiding position conflicts from prior edits)
- **Verify holistically**: After each modification, validate that interdependent parts (function signatures, return statements, call sites) remain synchronized to catch cascading errors early

**Underlying Insight**: Accept tool limitations as design constraints; pragmatism (working within bounds) beats attempting perfect solutions that exceed the tool's capabilities.

> id: 3c138d64-ba7e-456a-8758-7b633fcd2288

# Skill: Structured Code Manipulation Through AST-Based Tooling

**Core Pattern:** Using Abstract Syntax Trees (AST) as the authoritative representation for code transformations, rather than text-based manipulation. This involves:

1. **AST-First Architecture** — Parse code into structured trees, perform all modifications at the node level, and serialize back through canonical formatters. Eliminates text-based errors (line drift, malformed diffs, string fabrication).

2. **Tool Constraint Navigation** — Work within the invariants and limitations of AST tools (grv, Go's ast package) rather than against them. Develop workarounds for quirks (bootstrap patterns, multi-step sequences, file existence requirements) and design implementations within tool capability boundaries.

3. **Atomic + Fresh Operations** — Every write operation parses the file fresh (no caching), modifies the AST, and writes atomically via temp file + rename. Combined with readonly detection, this prevents silent corruption in concurrent or daemon scenarios.

4. **Scope Narrowing for Reliability** — Restrict processing to well-defined file types (`.go`, `go.mod`) and use multi-layered validation (extension lists, content scanning, non-text detection) to ensure safe, predictable tool behavior.

**Outcome:** Large-scale, zero-error refactoring and code generation with strong guarantees against corruption or malformation.

> id: 0b612c4b-9377-4a0b-a924-a6014737b961

Safe Go file write pattern: write to temp file in same directory, then os.Rename. Atomic on Linux/macOS for same filesystem. Pattern: tmp, _ := os.CreateTemp(dir, '.grv-*.go'); tmp.Write(content); tmp.Close(); os.Rename(tmp.Name(), target). Add defer os.Remove(tmp.Name()) as cleanup if rename fails. Never write directly to the target — partially-written files corrupt the codebase if the process is interrupted.

> id: d8781e16-f6ba-4b00-ad3d-8dca1ac60d46

# Skill: Safe AST-Based Code Modification with Comment Preservation

**Core Pattern**: Implement a parse-mutate-reattach-format-write cycle for programmatic code editing that prevents comment displacement and data corruption.

**Key Technical Components**:
1. **AST Mutation with Comment Reconciliation** – Parse code into an AST, apply mutations via callbacks, then explicitly reattach comments using tools like `ast.NewCommentMap().Filter()` *before* formatting to prevent comments from being misplaced when their associated nodes shift position.

2. **Atomic File Writes** – Use temp file + atomic rename patterns instead of direct overwrites to ensure data integrity if the process crashes mid-operation.

3. **Structured Error Context** – Surface failure points and available alternatives in error responses so clients can provide meaningful guidance rather than opaque error messages.

4. **Dry-Run Capability** – Support uniform dry-run parameters across all mutation operations to allow safe previewing of changes before committing.

**Why It Matters**: Naive AST modifications lose comment positioning and risk data corruption. This pattern solves both problems through deliberate comment reanchoring and atomic I/O—essential for tools that edit source code while preserving user formatting and documentation.

> id: 39817901-04ca-40a5-a6a9-0c31ff57522c

# Skill: Systematic AST Node Analysis and Serialization

**Core Pattern:** Design and implement comprehensive, modular systems for parsing, traversing, and serializing Abstract Syntax Trees (AST) with consistent metadata extraction and flexible querying mechanisms.

**Key Competencies:**
1. **Complete Coverage with Systematic Organization** — Map all node types in a language's AST into modular, categorized structures with dedicated handling logic for each type
2. **Standardized Metadata Extraction** — Define and apply consistent schemas (counts, flags, identifiers, structural properties) across diverse node types to enable uniform downstream processing
3. **Flexible Query Architecture** — Implement multi-level query patterns (single, batch, metadata-only) that navigate heterogeneous tree structures using context-aware path generation
4. **Graceful Handling of Edge Cases** — Address optional fields, missing data, parse errors, and structural variants through defensive design (validation, omitempty tags, null checks)
5. **Bidirectional Serialization** — Create reversible marshaling systems (`ToAST()`/`FromAST()`) that bridge internal representations with queryable formats (JSON) while maintaining type safety

**Application:** Building tools that make code structure programmatically accessible and manipulable across language ecosystems.

> id: 1b1b73bb-dff3-4438-8aba-0204ab4dafec


## Current Project Context
# Key Insights: AST Manipulation Tools Development

## 1. **Asymmetric API Design for Insert vs. Replace**
**Decision:** `ast_insert` supports flexible target routing (explicit file, directory auto-route, or daemon-injected working directory), while `ast_replace` requires an explicit file path.

**Rationale:** Insert operations benefit from intelligent placement logic (`HandleASTPlace`) to find canonical files within packages, reducing caller burden. Replace operations target specific, known locations and don't need this complexity.

**Implication:** This design reduces friction for insertion but requires callers to be more precise about replacement targets—a reasonable tradeoff given that replacements are typically targeted at known code locations.

## 2. **Post-Write Validation Hook for Safety**
**Problem Encountered:** Code modifications could introduce subtle breaking changes (type mismatches, syntax errors) that aren't caught during AST construction.

**Solution:** Added `enforcePostWrite()` check that validates the written file against original content using configurable checks (`DefaultChecksConfig.Enforce`) before confirming the operation.

**Implementation Detail:** Only enforces on actual writes (not dry-runs) and only when changes were made, minimizing performance overhead.

## 3. **Fallback Strategy for Insertion Context**
**Decision:** `insertIntoNode()` tries direct insertion into the target node first, then falls back to `insertIntoList()` via parent context if that fails.

**Problem Solved:** Some AST nodes are direct list containers (BlockStmt.List, FieldList), while others require parent-level manipulation. This dual approach handles both cases without requiring the caller to know the structural context.

**Benefit:** Simplifies the insertion logic and makes the tool more robust to different target node types.

> id: 94ca03a3-7cf0-42a0-889c-2dd83272b4f1

# Key Insights

## 1. **AST-Based Code Editing Eliminates String-Level Fragility**
The core problem: agents fail at code modification because they manipulate text positions/offsets that shift with edits. Solution: abstract to semantic operations on AST nodes instead. Agents express *what* to change (e.g., "add OR condition to if-statement in function X") rather than *where* (line numbers/string positions). This makes patches immune to offset drift and guarantees syntactic validity.

## 2. **Go Ecosystem Already Provides 90% of the Infrastructure**
Go's standard library (`go/ast`, `go/parser`, `go/printer`, `go/types`) and tools (`gopls`, `x/tools/go/analysis`) already operate internally on ASTs. The missing piece isn't parsing/analysis—it's the *write interface*: structured edit operations that agents can invoke to modify AST and auto-regenerate source. First-mate already does the read side; need to mirror it on the edit side.

## 3. **Practical MVP: Two-Command Interface with Git Diff as Source of Truth**
Build `lth ast query` (retrieve structured code elements) and `lth ast apply` (execute JSON-patch edits on AST, regenerate source). Agents never touch raw text; patches are validated by `git diff` on regenerated files. This approach sidesteps comment-preservation and formatting issues in real deployments while making SWE-bench-style hunk failures impossible.

> id: 17667c34-8811-4eed-b5e2-fd3a10c02bb2

# Key Insights

1. **Use Correct Tools for Go Files**: The `file_read` tool cannot process `.go` files directly. Always use `ast_*` tools (like `ast_node_at`, `ast_parse`) for Go code analysis instead.

2. **Proper ast_node_at Syntax Required**: The `ast_node_at` tool requires a specific format with `#DeclName` to identify the target declaration (e.g., `{"line":26,"col":1}#MyFunc`). Queries without this identifier will fail.

3. **Type Mismatches in AST Operations**: Ensure the AST node type matches the expected operation—attempting to extract a declaration from a `FuncType` node (which is a type definition, not a declaration) causes parsing failures. Always verify the node context before querying.

> id: 3e3e7273-212e-4bf2-a6a9-36a3157cf32c

# Key Insights from Go AST MCP Server Design

## 1. **Root Cause: Position-Based vs. Structure-Based Code Editing**
AI agents fail at code patching because they operate on text coordinates (line numbers, context strings) rather than semantic structure. The solution is a bidirectional AST interface where agents read code as JSON node trees and write by constructing those same trees—eliminating string manipulation, hunk offset errors, and fabricated diffs entirely.

## 2. **Structural Abstraction Eliminates Brittle Dependencies**
By exposing Go's `ast` package as a discriminated JSON schema (`"kind"` field + recursive node composition), the MCP server becomes the single source of truth for parsing, validation, formatting, and serialization. This decouples agents from Go syntax details and allows the server to catch invalid operations (e.g., unknown operators) before writing files.

## 3. **Higher-Level Queries Bridge Navigation and Mutation**
Position-based queries (line/column → node + path) and semantic queries (imports, definitions, implementations) translate human-readable editing gestures into structural operations—matching LSP semantics but returning AST data. This layers intuitive operations atop the core AST machinery without introducing positional brittleness.

> id: f1c8ea1e-75f1-4fdf-b927-17b475dd86ae

# Key Insights: grv Code Architecture

## 1. **Strict Separation of Concerns: AST-Only for Go, Raw Text for Others**
   - **Decision**: Go files use exclusively AST node trees (JSON with `kind` discriminator); non-Go files use raw text via `file_read`/`file_write`
   - **Problem Solved**: Prevents mixing source-text shortcuts with structural queries, ensuring consistent bidirectional representation
   - **Implication**: All Go modifications must go through AST tools—no source text shortcuts allowed

## 2. **Stateless, Fresh-Parse Architecture with Atomic Writes**
   - **Decision**: Every write operation parses the target file fresh (no in-memory AST cache); all writes are atomic (temp file + `os.Rename`)
   - **Problem Solved**: Eliminates race conditions and stale-state bugs; protects original files if formatting fails
   - **Implication**: Simpler concurrency model but trades caching efficiency for safety and correctness

## 3. **Readonly Protection Enforced at Operations Layer**
   - **Decision**: Readonly detection happens upfront before any write operation (checks `vendor/`, `GOROOT`, module cache, filesystem permissions)
   - **Problem Solved**: Prevents accidental modifications to protected directories/files
   - **Implication**: Clear, centralized enforcement prevents bypasses at higher layers

> id: ebbbd437-c296-4e7e-9629-971b0fb32473


## Related Context (via graph)
When building URL paths before passing to url.Parse, pre-escape # as %23 at the call site. url.Parse treats # as a fragment separator and silently strips everything after it. Fix: strings.ReplaceAll(link, "#", "%23") before url.Parse-based escape functions. Hugo fix: resources/page/page_paths.go, CreateTargetPaths.

> id: f0b618a8-d0dd-4450-845a-38df62acde7e

When set elements are bounded integers (e.g. block indices 0..N), replace map[int]struct{} with a dense bitset ([]uint64, words=ceil(N/64)). Intersection becomes bitwise AND, union becomes OR — no hash overhead, no per-bucket allocations. Return semantics: (bitset, bool) where bool=false means 'unconstrained/no-index' (keep-all), replacing nil-map convention. For N=32 blocks: 8 bytes vs multiple map bucket allocs.

> id: f4a4cf7f-1ad4-4471-8389-910350c7ab66

lth-grv workflow proof of concept: ran allocation optimization on blockpack entirely through grv tools — no Read/Write/Edit on source files. Agents used: grv ast_query to read structs/functions, grv ast_insert to add fields, grv ast_replace to change map key types, grv ast_find_symbols to locate call sites. The grv daemon injectDir middleware meant agents didn't need to specify file paths explicitly. The workflow successfully produced production-quality optimizations (57% alloc reduction, all tests pass) with zero direct file I/O. Key finding: grv is viable for real optimization work, not just exploration.

> id: 92be68b1-2da6-4d03-84dc-eb99caf2568a

# Key Insights

## 1. **AST-based code modification has fundamental limits with complex function bodies**
When attempting to insert large, multi-line function implementations using position-based AST tools (grv), formatting and comment placement breaks due to the tool's inability to track nested contexts. **Solution**: Extract complex logic into separate helper functions in new files, then use single-line assignments in the original file. This keeps AST modifications simple and predictable.

## 2. **Git stash restores can undo incremental progress**
Using `git stash` to fix a broken state reverted previously-added imports (bufio, io, sync) that were added via grv, requiring re-doing that work. **Solution**: When working around tool limitations, track all modifications separately and verify imports/dependencies are preserved after any state reset. Consider using grv to document dependency additions before complex edits.

## 3. **Tool constraint conflicts require pragmatic workarounds while respecting intent**
When grv's AST-based approach fails for legitimate code changes, the constraint to "use grv for all file access" can't be directly satisfied. **Solution**: Restructure the code to fit the tool's capabilities (extract to separate files, use simple assignments) rather than fighting the tool's limitations. This maintains the *spirit* of safe, auditable code changes while working within practical constraints.

> id: bcd6ce49-8094-4436-841a-32c8a677cd30

# Key Insights

## 1. **AST-First Architecture Solves Reliability**
The core problem—AI agents producing malformed diffs through text manipulation—is solved by enforcing bidirectional AST operations. No raw source text flows in or out; all code is represented as structured JSON node trees with a `"kind"` discriminator. This eliminates line-number drift, fabricated context, and string-based errors entirely.

## 2. **grv Tool Requires Careful Integration**
The `grv` binary is mature (latest at 6adb7c5+) but has constraints: (a) it cannot accept integer arguments from CLI for `ast_insert`—solution is to create helper functions in separate files and use `ast_replace` on statement blocks instead; (b) grv enforces strict invariants (no raw source in responses, fresh parsing on every write, atomic file writes, readonly detection at ops layer). Verify binary version and leverage available tools (`gomod_drop_require`, `gomod_replace`, `file_read/write` for non-Go files) rather than fighting CLI limitations.

## 3. **Write Operations Must Be Atomic and Fresh**
Every write tool must parse the target file fresh on each call (no in-memory cache) and write atomically via temp file + `os.Rename`. Combined with readonly detection (vendor/, GOROOT, module cache, filesystem permissions), this prevents silent corruption and concurrent-edit conflicts—critical for multi-agent or long-running daemon scenarios like `runWatchDaemon`.

> id: d6c8aab3-edf3-4145-835f-df5d1147480d


## Memory IDs (for exploration)
Use these IDs to explore further:
  lth get <id>                    — read full memory
  lth graph show --from <id>      — traverse graph edges
  lth graph ppr --seeds <id,...>  — personalized pagerank from seeds

  100abe24-403f-44cc-9c59-70d705dc2894
  ea804344-1d3b-4d3f-8352-dddb39d20269
  221ff919-1019-4f12-87b4-3e8cd7776bbe
  a18375ff-d18a-43dd-b347-81b5d9090645
  ec233da4-b145-49e4-a9cf-b7fb86808067
  3c138d64-ba7e-456a-8758-7b633fcd2288
  0b612c4b-9377-4a0b-a924-a6014737b961
  d8781e16-f6ba-4b00-ad3d-8dca1ac60d46
  39817901-04ca-40a5-a6a9-0c31ff57522c
  1b1b73bb-dff3-4438-8aba-0204ab4dafec
  94ca03a3-7cf0-42a0-889c-2dd83272b4f1
  17667c34-8811-4eed-b5e2-fd3a10c02bb2
  3e3e7273-212e-4bf2-a6a9-36a3157cf32c
  f1c8ea1e-75f1-4fdf-b927-17b475dd86ae
  ebbbd437-c296-4e7e-9629-971b0fb32473
  f0b618a8-d0dd-4450-845a-38df62acde7e
  f4a4cf7f-1ad4-4471-8389-910350c7ab66
  92be68b1-2da6-4d03-84dc-eb99caf2568a
  bcd6ce49-8094-4436-841a-32c8a677cd30
  d6c8aab3-edf3-4145-835f-df5d1147480d

## Filter by project
Memories from these projects are present:
  lth prompt "..." --attr project=mattdurham/grv
  lth projects  — list all tracked projects
  lth chat "..." --attr project=<project> — filtered chat
