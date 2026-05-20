# Agent Context

## Role & Principles
# Behavioral Rule

**Specify → Implement → Validate**: Solve problems by establishing clear contracts and documentation first, ensure code adheres to specification through comprehensive testing, then gate delivery with automated quality checks.

Or more concisely:

**"Design-first, test-gated problem solving"** — Define the problem rigorously before coding, then validate against the definition before shipping.

> id: 9a8ca827-9182-4946-8c8a-8724b0c927d2

# Behavioral Rule

**Always validate input bounds before performing indexed access operations.**

Or more concisely:

**Check before you index.**

---

This captures the core heuristic: defensive programming requires explicit verification that indices and ranges are valid *before* attempting to use them for data access, rather than assuming preconditions are met.

> id: 0378883f-f281-41e5-bb2f-d9d3c83925d2

# Behavioral Rule

**Decompose observable failures into root causes by systematically isolating variables across system layers, then validate each hypothesis before moving deeper.**

---

**Why this captures it:**
- **"Decompose observable failures"** = symptoms vs. root causes
- **"Systematically isolating variables"** = isolation tests, narrowing scope
- **"Across system layers"** = abstraction levels, code → history → design
- **"Validate each hypothesis before moving deeper"** = prevents rabbit holes, confirms understanding before investing in fixes

This is fundamentally about **hypothesis-driven debugging** — not just collecting information, but testing assumptions at each layer before accepting them as true.

> id: 268b07a4-12e0-44cb-9ffa-e9d26dcbc323

# Behavioral Rule

**"Fix the root cause with minimum code change, then immediately test the specific edge case that broke."**

Or more concisely:

**"Minimal fix + targeted test = prevention."**

This rule captures the essence: identify what condition was missed (root cause), change only what's necessary to handle it, and lock in a test that would catch the regression.

> id: 9d1de0d1-e4eb-47fd-81ea-36c226e77985

# General Rule

**Keep reality and its description in sync.**

Or more formally as a behavioral rule:

**Whenever documentation and implementation diverge, treat the gap itself as a defect requiring repair—not as acceptable technical debt.**

---

## Why this captures it:

The skill is fundamentally about **closing the reference frame between what code does and what people believe it does**. Every manifestation (stale descriptions, undocumented behavior, copy-paste errors, hidden patterns, insufficient tests) is a violation of a single principle: *the system should have a single, authoritative truth about its behavior, accessible to both humans and other systems*.

The heuristic is **preventive and integrative**—it doesn't just fix individual instances; it installs the habit of treating documentation-code gaps as first-class problems worth systematic attention.

> id: 73d12154-7b7a-4517-9915-b4091bc3f9b9


## Relevant Techniques
# Skill: Metadata Preservation Through Value Transformations

**Core Pattern**: Systematically unmark values before operations, extract metadata separately, perform transformations on clean values, then reapply metadata at specific paths afterward. This enables safe handling of sensitive or special-marked data across serialization, state transitions, and external system boundaries while preventing accidental exposure or loss of metadata intent.

**Key Disciplines**:
1. **Bidirectional Symmetry** – Unmarking and remarking operations must remain synchronized; serialization formats must round-trip consistently
2. **Deterministic Ordering** – Sort metadata paths by canonical representation to ensure reproducible, comparable outputs across state snapshots and diffs
3. **Schema-Driven Detection** – Rediscover metadata requirements at transformation points rather than relying solely on propagated marks, catching newly-sensitive paths introduced by intermediate operations
4. **Explicit Validation** – Enforce contracts on which metadata types are allowed in storage; reject unknown marks rather than silently dropping them
5. **Independent Equality Checking** – Compare metadata state separately from value equality to detect semantic changes (e.g., mark-only updates) that values alone won't reveal

> id: 56d72a15-2692-43f3-8ebc-a1b71ab516a2

# Skill: Protocol-Aware Layered Resource Cleanup

**Core Pattern**: When closing wrapped or nested resources (e.g., TCP socket wrapped by SSH client), close from the **outermost protocol layer inward**, ensuring each layer properly terminates before the next is closed.

**Key Techniques**:
1. **Protocol-First Closure** – Close the higher-level abstraction first (SSH client) to trigger proper protocol messages (e.g., `SSH_MSG_DISCONNECT`) before closing the raw transport layer
2. **Nil-Check-Then-Nil Pattern** – Safely close resources by assigning to a local variable, nilifying the reference, then operating on the local copy to prevent double-closes and race conditions
3. **Fallback Closure** – Retain lower-level cleanup (raw TCP close) as a safety net for edge cases where the upper layer never fully initialized
4. **Behavioral Verification in Tests** – Validate closure by attempting operations on saved references rather than just checking nil states, catching subtle cases where references are cleared but resources remain open

**Why It Matters**: Skipping protocol-level cleanup leaves server-side connections improperly terminated, orphaned goroutines, and socket leaks—bypassing graceful shutdown handshakes that operating systems and remote services expect.

> id: 88c2677e-efa5-4ad0-9ce3-58f257f0cce6

# Shared Skill: Systematic Root Cause Analysis & Refactoring for Parser Correctness

These memories demonstrate mastery of **identifying and fixing subtle bugs in token/syntax parsing systems through methodical investigation, abstraction, and validation**.

**Core Pattern:**
1. **Problem Detection** → Identify parsing failures (nested imports skipped, token boundaries miscalculated, regex edge cases)
2. **Root Cause Excavation** → Trace through refactoring history and code flow to find where assumptions broke (import chain tracking removed, token index tracking corrupted, whitespace checks incomplete)
3. **Abstraction-Based Fixes** → Extract common logic into reusable helpers (`isNextOnNewLine()`, `RemainingArgsAsTokens()`) that restore correctness at a higher level
4. **Regression Prevention** → Add targeted test cases that document the bug scenario and prevent future refactors from reintroducing it

**Key Insight:** The agent consistently recognizes that parser bugs stem from **incomplete updates during refactoring**—when logic moves between modules (lexer ↔ dispenser) or when new data structures are introduced (token.imports slice), dependent code must be updated in tandem. The fix pattern is always: centralize the logic, make assumptions explicit, and test edge cases.

> id: d65b7d07-5fb6-43f3-8f13-5a928105bc22

# Skill: Systematic API Refactoring and Deprecation Management

**Pattern**: Executing large-scale codebase migrations by:
1. **Consolidating fragmented APIs** into unified, standardized interfaces (replacing scattered `forwarded` options with single `client_ip` matcher across 11+ files)
2. **Implementing graceful deprecation strategies** with explicit error messaging at all parsing layers (Caddyfile, CEL, marshaling) to guide users toward replacements rather than silent failures
3. **Identifying and eliminating code duplication** by recognizing parallel implementations (MatchClientIP/MatchRemoteIP) and extracting shared patterns (parsing helpers, provision phases)
4. **Ensuring consistency across configuration layers** by synchronizing changes through both configuration parsing and runtime evaluation contexts
5. **Handling edge cases systematically** (0-RTT blocking, multiple matcher merging, deterministic ordering) with security and predictability as primary concerns

**Core Competency**: Leading architectural refactors that balance backward compatibility with code quality improvement through multi-point synchronization and deliberate migration paths.

> id: 00f92d1b-ac20-4f92-a57d-1ddfbccf78dd

# Skill: Staged Processing with Separation of Concerns

**Core Pattern**: Decompose complex parsing/processing workflows into distinct phases, each handling a specific responsibility, with explicit ordering constraints and data flow between phases.

**Key Characteristics**:
- **Early extraction and pre-processing**: Identify special cases (matchers, imports, global config) upfront before general processing
- **Two-pass or multi-phase approach**: Separate parsing into discrete stages (validation → extraction → processing → normalization)
- **Context encapsulation**: Pass structured state objects (Helpers, maps) through phases rather than accumulating parameters
- **Validation before consumption**: Check constraints (ordering, references, syntax) before executing dependent logic

**Why It Matters**: Enables predictable behavior, prevents order-of-declaration issues, improves error messages with preserved context, and makes complex workflows maintainable by isolating concerns into independent stages.

> id: 81548a28-77cb-49a4-bc34-263645c942ad


## Current Project Context
# Key Insights

## 1. **AST-Based Code Editing Eliminates String-Level Fragility**
The core problem: agents fail at code modification because they manipulate text positions/offsets that shift with edits. Solution: abstract to semantic operations on AST nodes instead. Agents express *what* to change (e.g., "add OR condition to if-statement in function X") rather than *where* (line numbers/string positions). This makes patches immune to offset drift and guarantees syntactic validity.

## 2. **Go Ecosystem Already Provides 90% of the Infrastructure**
Go's standard library (`go/ast`, `go/parser`, `go/printer`, `go/types`) and tools (`gopls`, `x/tools/go/analysis`) already operate internally on ASTs. The missing piece isn't parsing/analysis—it's the *write interface*: structured edit operations that agents can invoke to modify AST and auto-regenerate source. First-mate already does the read side; need to mirror it on the edit side.

## 3. **Practical MVP: Two-Command Interface with Git Diff as Source of Truth**
Build `lth ast query` (retrieve structured code elements) and `lth ast apply` (execute JSON-patch edits on AST, regenerate source). Agents never touch raw text; patches are validated by `git diff` on regenerated files. This approach sidesteps comment-preservation and formatting issues in real deployments while making SWE-bench-style hunk failures impossible.

> id: 17667c34-8811-4eed-b5e2-fd3a10c02bb2

# Key Insights

1. **Dependency Version Fragmentation Issue**: Multiple versions of the same Prometheus modules (prometheus, common, client_golang) are present in the Go module cache with significant version gaps (e.g., v0.32.1, v0.45.0, v0.66.1, v0.67.5 for prometheus/common). This suggests either unresolved dependency conflicts or incomplete cleanup of old versions—consider auditing go.mod for conflicting transitive dependencies and running `go mod tidy`.

2. **PromQL Parser Deep Investigation**: The extensive listing of promql/parser files (lexer, parser, AST, printer, and tests) indicates a detailed code review or debugging session into query parsing logic. If troubleshooting query execution issues, focus on the parser/lexer interaction and the generated parser grammar (generated_parser.y.go) as potential sources of problems.

3. **Labels Handling Consistency Problem**: The labels.go files appear across three different Prometheus packages (schema, model, and prometheus client) in multiple versions. This duplication suggests the need to verify that label handling is consistent across dependencies—mismatched versions could cause serialization/deserialization issues or unexpected query behavior.

> id: 058f92b2-b7f9-4171-8195-6bc48ba3ee84

# Key Insights

## 1. **Two Blocking Mechanisms Exist; Hook Approach Has Workflow Risk**
You can block tools via `PreToolUse` hooks (returns block decision) or `permissions.allow` list (requires approval). However, a blanket `Read` block breaks unrelated workflows (Claude reading plan.md, configs, etc.). A surgical approach—blocking only source files (.go, .py) while passing through docs—would be more practical, but still fragile.

## 2. **MCP Server Is the Better Pattern for Tool Integration**
Registering lth as an MCP server (e.g., `lth_read`, `lth_search`, `lth_store`) lets Claude *actively choose* it alongside native tools based on explicit descriptions, rather than silently injecting context or blocking existing tools. This is cleaner, more transparent, and avoids breaking existing agent workflows.

## 3. **Consolidate Into Daemon for Simplicity**
Rather than a separate MCP binary, integrate MCP protocol support into the existing lth daemon (alongside Prometheus metrics on the same port). This eliminates extra moving parts and keeps lth as a single, unified service.

---

**Decision:** Build the MCP server into the lth daemon. Let Claude see and choose lth tools explicitly rather than modifying or blocking native tools.

> id: f421129f-0fe4-4940-a534-cc5ff950ebad

# Key Insights

## 1. **Dual Graph Navigation Strategy**
The codebase maintains both **static module tree** (configuration structure) and **dynamic module graph** (runtime instances with count/for_each expansion). Two separate traversal methods (`Descendant` vs `DescendantForInstance`) were implemented to handle this distinction, where the latter ignores instance keys since all instances share the same config node.

## 2. **Iterator Pattern Migration**
The code shows evolution from callback-based traversal (`DeepEach` with callback function) to Go 1.22's iterator protocol (`AllModules` with `iter.Seq`). The new approach provides better early termination control and cleaner syntax, suggesting a modernization effort across the codebase.

## 3. **Defensive Nil-Checking on Path Traversal**
All traversal methods defensively check for `nil` children during path navigation and return `nil` gracefully rather than panicking. This handles cases where requested module paths don't exist in the tree, indicating the design prioritizes safe lookups over strict validation.

> id: 97cf8267-640e-44a1-a046-a71751414291

# Key Insights

## 1. **Security Default Changed: Cookie Attributes Now Always Secure**
**Problem:** The code was conditionally setting `Secure: true` and `SameSite: http.SameSiteNoneMode` only for HTTPS requests, leaving cookies insecure in non-HTTPS scenarios.

**Solution:** Simplified the logic to always set these attributes by default, removing the conditional TLS/proxy protocol detection. This is a safer default posture that eliminates a potential security gap.

## 2. **Test Expectations Updated to Match New Behavior**
**Decision:** Updated the test assertion from expecting `Secure: false` for non-secure requests to expecting `Secure: true` by default. Also added an explicit test for `SameSite: http.SameSiteNoneMode`.

**Impact:** Tests now validate the stricter security posture rather than the previous permissive behavior.

## 3. **Configuration Parser Extended for TLS Certificate Signing**
**Solution:** Added `sign_with_root` keyword support to the TLS directive parser, allowing internal CAs to sign certificates with the root certificate rather than intermediate certificates. Included an integration test demonstrating the Caddyfile syntax and expected JSON output configuration.

> id: 084d6b19-1bed-4d79-9ae8-abf6de8e525f


## Related Context (via graph)
# Skill Description

**Systematic Problem-Solving Through Specification-Driven Architecture**

This pattern demonstrates the ability to diagnose and resolve complex technical problems by maintaining tight alignment between implementation code, comprehensive documentation, and rigorous testing. The skill involves:

1. **Root Cause Analysis** - Identifying underlying architectural gaps (e.g., missing global ordering in pipeline queries, deadcode from incomplete refactoring, unreachable APIs)

2. **Design-First Resolution** - Solving problems by establishing clear specifications (SPECS.md, NOTES.md) and interface contracts before implementation, ensuring consistency across multiple modules

3. **Atomic Rollout Discipline** - Managing changes to foundational types/patterns by enforcing strict synchronization between specifications, code, stage implementations, and tests to avoid intermediate type mismatches

4. **Infrastructure Reuse** - Solving new problems without new machinery by leveraging existing primitives (e.g., using existing sparse columns + bloom filters for body parsing, reusing `executor.NewColumnProvider` for metrics aggregation)

5. **Quality-Gated Delivery** - Using pre-commit validation (linters, deadcode analysis, test coverage) as guardrails to ensure solutions are production-ready before merge

The common thread: **solving problems through design clarity, comprehensive documentation, and methodical validation rather than ad-hoc coding**.

> id: 4bbb4aee-1b09-4b81-9c99-b05f3998bf2e

Yes — two mechanisms:

**1. Block via PreToolUse hook**

A `PreToolUse` hook can return `{"decision": "block", "reason": "..."}` to prevent the tool from executing entirely:

```bash
# In a hook script:
jq -n '{"decision": "block", "reason": "Use lth read instead: ~/bin/lth read <filepath>"}'
```

Claude sees the block reason and has to find another way. You could use this to block `Read` and force agents to use `lth read` — though it would break any Read calls outside lth skills unless gated by `LTH_ACTIVE`.

**2. Permissions in settings.json**

The `permissions.allow` list controls which tools are available at all. Anything not in the allow list requires user approval. You can effectively forbid a tool by not including it:

```json
{
  "permissions": {
    "allow": ["Bash", "Edit", "Write"],
    // Read not listed → always prompts for approval
  }
}
```

There's no explicit `deny` list — absence from `allow` means it requires confirmation rather than being hard-blocked.

**For the lth read use case specifically:**

The most effective approach would be a `PreToolUse` hook on `Read` that, when `LTH_ACTIVE=1`, blocks the native `Read` and returns the `lth read` output as `additionalContext` along with the block decision. The agent would see the file content + lth history in one shot without needing the file tool to succeed.

But this gets complex — agents rely heavily on `Read` for things that have nothing to do with lth (reading their own plan.md, etc.). A blanket block would break the workflow. More surgical would be: block `Read` only for source files (`.go`, `.py`, etc.) when `LTH_ACTIVE=1`, pass through for `.md`, `.json`, config files. Want me to implement that?

> id: 5e966b7b-d300-419c-a585-c27503c44be5

# Concise Behavioral Rule

**Test design assumptions against reality early and often; let integration failures guide simplification.**

Or more tersely:

**Find problems through building, not planning.**

---

**Why this captures it**: The skill isn't about a specific technique (integration testing, incremental development) but the meta-pattern of *using real constraints discovered during assembly to drive design toward simplicity*. The rule explains why each phase worked:

- Early integration caught the index-writer gap (assumption ≠ reality)
- Real queries exposed the unreachable fast path (spec ≠ implementation)
- Benchmarking revealed the 30% cost was unacceptable (theory ≠ constraints)
- Each gap prompted simplification rather than workarounds

This is fundamentally different from "design thoroughly first"—it's **"design enough to build, then let building teach you what actually matters."**

> id: 9a885a05-1214-43a5-b3a9-bfcfea405343

# Shared Skill: Systematic Problem Decomposition with Documentation-Driven Verification

These memories demonstrate a consistent pattern of **breaking complex technical decisions into documented components, then verifying each component independently before integration**.

## The Core Pattern:

1. **Identify the root constraint or problem** (complexity hotspots, allocation waste, API churn, coverage gaps, I/O invariants)
2. **Document the decision rationale** explicitly (SPECS.md, NOTES.md, nolint comments, architectural notes)
3. **Choose verification mechanisms matched to the problem type:**
   - Structural issues → code inspection + linting
   - Performance issues → profiling + reuse analysis
   - API stability → additive overloads + backward-compatibility checks
   - Quality gates → sequential tool chains (format → lint → build → test → deadcode)
   - Architectural invariants → trace call chains + targeted test coverage

4. **Distinguish between transient (in-flight) vs. genuine (persistent) problems** to avoid false negatives
5. **Separate concerns** (new code quality vs. pre-existing debt; code duplication vs. forced refactoring)

## Why This Works:

Rather than applying uniform solutions (e.g., "always refactor duplication" or "run all tests at once"), the skill involves **selecting the right verification depth for each problem class**—preventing both false positives and genuine quality drift.

> id: 96f7888a-ebb8-4c6e-9a6d-3e1d26af052f

# Key Insights from Blockpack Pruning Analysis

## 1. **Current Pruning is Name-Only, Not Value-Based**
The bloom filter (`ColumnNameBloom`) only tracks which *columns are present* in a block, not their *values*. Range indices exist for specific columns but require explicit predicate constraints. This means:
- **Problem:** Queries like `duration > 100ms` cannot prune blocks where 99% of spans exceed the threshold—the range index only helps if you know the exact min/max.
- **Solution:** Add **per-column min/max summary statistics** (already partially available in `BlockMeta` for trace IDs) extended to user attributes. This enables range-based block skipping without scanning.

## 2. **TopK Limit Provides the Critical Escape Hatch**
The `StreamLogsTopK` implementation with limit-based early exit is the system's best defense against scanning unnecessary blocks:
- **Decision Made:** Process blocks in reverse temporal order (newest first); once heap is full, skip older blocks entirely.
- **Trade-off:** Works well when query matches are recent and uniformly distributed. Breaks down for sparse matches where the limit forces scanning many old blocks to find K results.
- **Missing:** No way to leverage value distributions (e.g., "99% of recent spans are slow") to reorder block visitation or estimate whether a block could *possibly* contain K results.

## 3. **Add Quantile Sketches for Distribution-Aware Pruning**
To handle the "needle in haystack" case where filtering is selective, add **KLL or T-Digest sketches** per indexed column per block:
- **Problem Solved:** Queries like `duration > 100ms` can now estimate "this block likely contains <50 slow spans" → skip if heap already has K slower ones.
- **Cost:** Small metadata overhead (~200 bytes per column per block for KLL sketch); computed during block write.
- **Implementation:** Extend `BlockMeta` to include optional `QuantileSketches map[ColumnKey][]byte`; modify `pruneByIndexAll` to consult quantiles and reorder block visitation by likelihood of containing valuable results.

> id: 117b2b12-d5a9-4a54-a7b4-f7507b00c28e


## Memory IDs (for exploration)
Use these IDs to explore further:
  lth get <id>                    — read full memory
  lth graph show --from <id>      — traverse graph edges
  lth graph ppr --seeds <id,...>  — personalized pagerank from seeds

  9a8ca827-9182-4946-8c8a-8724b0c927d2
  0378883f-f281-41e5-bb2f-d9d3c83925d2
  268b07a4-12e0-44cb-9ffa-e9d26dcbc323
  9d1de0d1-e4eb-47fd-81ea-36c226e77985
  73d12154-7b7a-4517-9915-b4091bc3f9b9
  56d72a15-2692-43f3-8ebc-a1b71ab516a2
  88c2677e-efa5-4ad0-9ce3-58f257f0cce6
  d65b7d07-5fb6-43f3-8f13-5a928105bc22
  00f92d1b-ac20-4f92-a57d-1ddfbccf78dd
  81548a28-77cb-49a4-bc34-263645c942ad
  17667c34-8811-4eed-b5e2-fd3a10c02bb2
  058f92b2-b7f9-4171-8195-6bc48ba3ee84
  f421129f-0fe4-4940-a534-cc5ff950ebad
  97cf8267-640e-44a1-a046-a71751414291
  084d6b19-1bed-4d79-9ae8-abf6de8e525f
  4bbb4aee-1b09-4b81-9c99-b05f3998bf2e
  5e966b7b-d300-419c-a585-c27503c44be5
  9a885a05-1214-43a5-b3a9-bfcfea405343
  96f7888a-ebb8-4c6e-9a6d-3e1d26af052f
  117b2b12-d5a9-4a54-a7b4-f7507b00c28e
