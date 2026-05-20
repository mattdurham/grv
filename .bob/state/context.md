# Agent Context

## Role & Principles
Run background processes asynchronously using goroutines with context management

> id: 8e3b88bc-8a6c-4b2d-9eb2-17fa12d891de

# Behavioral Rule

**Decompose complex systems into stateless, composable units that communicate through immutable data structures rather than shared state or modified signatures.**

---

## Why This Captures the Pattern

- **Stateless + composable** = the core architectural choice
- **Immutable data + explicit communication** = the mechanism enabling it
- **Rather than shared state/signature changes** = contrasts with the anti-patterns avoided

This generalizes beyond pipelines to any domain requiring concurrency, testability, and reusability (streaming systems, functional middleware, event processors, etc.).

> id: eaa02943-9b77-4f28-92fd-3d0e31db92ef

Monitor daemon state changes and restart with updated binaries to maintain system live status

> id: cee9f7b5-6143-454d-8751-207ba857b06c

# General Rule

**Verify implementation against intent through layered validation, then document failure modes to prevent recurrence.**

Or more concisely:

**Test → Fix → Teach → Prevent**

---

## Why this captures it:

1. **Test** = systematic gap identification (code review, build checks, integration tests)
2. **Fix** = scoped remediation with multi-layer confirmation
3. **Teach** = context enrichment and anti-pattern documentation
4. **Prevent** = architectural consistency checks that block future instances

This heuristic generalizes across code quality, guidance accuracy, system design, and documentation—anywhere intent can diverge from execution.

> id: c121b38e-136d-4758-bf98-5c481e17d6d3

# Behavioral Rule

**"Fix the root cause with minimum code change, then immediately test the specific edge case that broke."**

Or more concisely:

**"Minimal fix + targeted test = prevention."**

This rule captures the essence: identify what condition was missed (root cause), change only what's necessary to handle it, and lock in a test that would catch the regression.

> id: 9d1de0d1-e4eb-47fd-81ea-36c226e77985


## Relevant Techniques
# Skill: Asynchronous Task Orchestration with File-Based Progress Tracking

**Core Pattern**: Execute long-running operations in the background using unique task IDs and persistent output logging to temporary files, enabling non-blocking workflow continuation and interim progress monitoring without polling task status directly.

**Key Components**:
- Background task execution for parallel/concurrent processing
- Unique task identification and output file mapping
- File-based progress visibility and completion verification
- Exit code validation for success confirmation

**Value**: Improves efficiency and responsiveness by decoupling long-running operations from main workflow while maintaining full observability into task progress and results.

> id: 64b304cc-af31-41d6-97bb-14341314e6e2

# Skill: Defensive Concurrent Resource Lifecycle Management

**Core Pattern:** Design resource cleanup sequences that are thread-safe, order-dependent, and resilient to partial failures through explicit state nullification and defensive nil-checks.

**Key Techniques:**
1. **Ordered Protocol-Level Cleanup** – Release resources in reverse dependency order (cancel goroutines → close protocol layer → close sockets) with explicit nil-assignment after each step to prevent double-closes and stale references
2. **Mutex + Local Reference Pattern** – Protect shared state with locks and capture pointer snapshots before passing to goroutines, preventing race conditions during concurrent reconnection or reallocation
3. **Graceful Degradation via Nil-Checks** – Check resources before closing and allow partial cleanup success, enabling the system to handle incompletely-initialized or partially-failed connections without cascading failures

**Context:** Essential for production systems with long-running background goroutines (keepalive, provisioning) where timing between cleanup operations and concurrent access determines whether resources leak, hang, or panic.

> id: 072ffffa-aab2-48c1-ab22-f2cbe4d83f9a

# Skill: Protocol-Aware Layered Resource Cleanup

**Core Pattern**: When closing wrapped or nested resources (e.g., TCP socket wrapped by SSH client), close from the **outermost protocol layer inward**, ensuring each layer properly terminates before the next is closed.

**Key Techniques**:
1. **Protocol-First Closure** – Close the higher-level abstraction first (SSH client) to trigger proper protocol messages (e.g., `SSH_MSG_DISCONNECT`) before closing the raw transport layer
2. **Nil-Check-Then-Nil Pattern** – Safely close resources by assigning to a local variable, nilifying the reference, then operating on the local copy to prevent double-closes and race conditions
3. **Fallback Closure** – Retain lower-level cleanup (raw TCP close) as a safety net for edge cases where the upper layer never fully initialized
4. **Behavioral Verification in Tests** – Validate closure by attempting operations on saved references rather than just checking nil states, catching subtle cases where references are cleared but resources remain open

**Why It Matters**: Skipping protocol-level cleanup leaves server-side connections improperly terminated, orphaned goroutines, and socket leaks—bypassing graceful shutdown handshakes that operating systems and remote services expect.

> id: 88c2677e-efa5-4ad0-9ce3-58f257f0cce6

# Skill: Defensive Nil-Safety in Conditional Component Initialization

**Core Pattern:**
When components are conditionally initialized based on application mode or configuration, design their cleanup/lifecycle methods to safely handle nil receivers rather than scattering nil checks across all call sites.

**Key Principle:**
Make cleanup methods idempotent no-ops when called on uninitialized (nil) components. This decouples initialization logic from shutdown logic and prevents crashes when teardown paths don't track which mode/state the application is in.

**Implementation Pattern:**
```go
func (ng *Engine) Close() error {
    if ng == nil {
        return nil  // Safe no-op for uninitialized components
    }
    // ... cleanup logic
}
```

**When to Apply:**
- Public lifecycle methods (`Close()`, `Shutdown()`, `SetQueryLogger()`) on optionally-initialized resources
- Components that vary by application mode (agent vs. normal mode)
- Cleanup code called unconditionally during shutdown paths

**Why It Matters:**
- Eliminates hidden nil cases in conditional initialization architectures
- Reduces cognitive load on callers (no need to track initialization state)
- Follows idiomatic Go patterns for defensive pointer receivers
- Requires complementary test coverage for nil and non-nil cases

> id: 61a1b905-a455-4187-a66d-815ef67d7644

# Skill: Dependency Chain Management & Unblocking Strategy

**Core Pattern**: Identifying critical path blockers in sequential work, synchronizing task status updates with deliverables, and actively unblocking downstream work through either:
- **Explicit completion signaling** (status updates matched to actual deliverables)
- **Productive waiting** (studying patterns/assumptions while blocked rather than idling)
- **Early validation** (pre-wiring reviews, specification-first approaches, and pre-handoff verification)

**Key Competencies**:
1. **Map blocking relationships** – Recognize when single-task failures cascade and cause idle time
2. **Decouple status from progress** – Distinguish between task completion and dependency readiness; update both
3. **Parallelize where possible** – Create minimal viable specs/scaffolds early to reduce sequential bottlenecks
4. **Validate before handoff** – Verify actual implementation against specs via code review or grepping, not task labels
5. **Communicate blockers proactively** – Surface idle time and dependency gaps early to keep pipelines moving

> id: 2a026a53-856e-438f-bd25-6f9bb87f6aca


## Current Project Context
# Key Insights

## 1. **Asynchronous Cleanup Caused Resource Leaks**
   - **Problem**: Both provisioners used goroutines to defer `comm.Disconnect()` until context cancellation, leaving SSH connections open after commands completed.
   - **Impact**: Connection resources weren't cleaned up synchronously, causing them to linger until goroutine scheduling.

## 2. **Solution: Replace Async Context Monitoring with Defer Pattern**
   - **Decision**: Replaced context-listening goroutines with simple `defer comm.Disconnect()` statements.
   - **Benefit**: Guarantees synchronous cleanup at function exit, eliminating the race condition between command completion and actual disconnection. This is more reliable and simpler than waiting for context cancellation.

## 3. **Simplified Control Flow**
   - **Removed**: Unnecessary context wrapping (`context.WithCancel`) and goroutine spawning in `remote-exec`.
   - **Result**: Cleaner code that directly manages resource lifecycle without relying on goroutine scheduling timing.

> id: 7110fd81-9ad3-4c5d-8ce9-0eb7aa1c4f92

# Key Insights: SSH Communicator Connection Management

## 1. **Nil-Check Pattern for Safe Resource Cleanup**
**Decision:** Set `c.conn = nil` after closing connections rather than relying solely on Close() operations.
**Value:** Prevents double-close errors and ensures subsequent nil-checks catch stale references—essential for reconnection logic where old connections must be explicitly nullified before new ones are established.

## 2. **Race Condition Prevention Through Local References**
**Problem:** Long-running keepalive goroutines could race with connection reconnects.
**Solution:** Use sync.Mutex on the Communicator struct and create local copies of pointers (e.g., `sshClient := c.client`) before passing into goroutines. This prevents goroutines from accessing reallocated pointers during reconnection.

## 3. **Two-Layer Liveness Detection Over Simple Ping/Pong**
**Decision:** Implement both periodic SendRequest + response timeout rather than basic heartbeat.
**Value:** Detects both dead connections (no response) and stuck connections (response timeout). Goroutines can independently close connections when liveness fails, preventing indefinite hangs during long-running provisioning operations.

> id: 60f0d84f-49c9-45e6-87ef-a70640c67b03

# Key Insights

## 1. **Strict CLI Contract & Error Handling**
The codebase enforces invariants around output formatting and error reporting: JSON output must always be valid (even on partial failures), all errors go to stderr, and non-zero exit codes are mandatory on failure. This ensures reliable scripting and tool integration.

**Decision**: Maintain these invariants rigorously across all commands to preserve CLI predictability.

## 2. **Daemon Initialization Consistency**
All DB-touching commands require `ensureDaemon` to be called first, with explicit exceptions only for `watch` and `config` subcommands. This prevents accidental daemon state issues.

**Decision**: Document and enforce this pattern as a code review requirement; consider using a wrapper or middleware to make it implicit.

## 3. **Metrics Integration Requires Store Access**
A recent change shows the metrics server now needs access to the daemon store (`metrics.NewServer(metricsAddr, reg, daemon.store)`), indicating monitoring capabilities are being tied to live runtime state.

**Problem encountered**: Metrics server was previously decoupled from store data.
**Solution found**: Pass store reference explicitly to enable real-time metrics reporting.

> id: 58212840-119f-4723-8068-7b0c1c652ce6

# Key Insights: SSH Communicator Implementation

## 1. **Thread-Safe Shared RNG with PID Multiplier**
**Decision:** Use a single global RNG seeded with `time.Now().UnixNano() * os.Getpid()`, protected by mutex lock.
**Problem:** Multiple RNG instances created simultaneously produce identical sequences; concurrent processes can write to the same files.
**Solution:** Share one RNG instance across all calls and multiply seed by PID to ensure process-unique sequences, preventing collisions in parallel operations.

## 2. **Connection Lifecycle Management with Cleanup**
**Decision:** Store net.Conn and ssh.Client as mutable state with explicit lock-protected initialization in Connect().
**Problem:** Stale connections need cleanup; concurrent access to connection state could cause data races.
**Solution:** Clear both conn and client to nil before recreation, use sync.Mutex on Communicator struct, and defer lock release to ensure proper cleanup even on error.

## 3. **Graceful Degradation via Custom Error Type**
**Decision:** Implement a `fatalError` wrapper type with `FatalError()` method.
**Problem:** Need to distinguish recoverable vs. unrecoverable SSH failures for appropriate retry logic.
**Solution:** Use type assertion pattern where fatal errors implement a specific interface, allowing callers to handle critical failures (host key verification, auth) differently from transient issues.

> id: 76df40f7-c558-4492-abb4-5ea3a433ccc9

# Key Insights

1. **Protocol-layer cleanup must precede transport-layer cleanup**: The SSH client object must be explicitly closed before the underlying TCP socket to ensure the SSH protocol sends its disconnect message (SSH_MSG_DISCONNECT). Closing only the raw socket leaves the application-level connection in an undefined state from the OS perspective, causing ESTABLISHED sockets to linger.

2. **Layered resource cleanup requires proper ordering and nil-safety**: When closing nested resources (SSH client wrapping TCP connection), close the outer layer first, nil the reference to prevent double-closes, then close the inner layer. Capture errors from the protocol close but ensure the transport close always executes (using a local error variable rather than early return).

3. **Missing explicit close calls are a common source of connection leaks**: The original code relied on implicit cleanup through raw socket closure, which is insufficient for stateful protocols. Always explicitly close high-level client objects even when they wrap lower-level connections—the library maintainer cannot assume the socket close will trigger protocol-level cleanup.

> id: 986fcfb4-1518-479d-b5cb-235419cda00c


## Related Context (via graph)
# Key Insights

## 1. **Root Cause: Missing SSH Protocol-Level Disconnect**
The `Disconnect()` method closed the raw TCP socket without calling `c.client.Close()`, skipping the SSH protocol-level disconnect (`SSH_MSG_DISCONNECT`). This left TCP sockets in ESTABLISHED state indefinitely because neither side sent FIN packets to the OS.

## 2. **Impact Multiplier: Resource Creation Pattern**
The problem was amplified by Terraform's design—each `null_resource` with `remote-exec` provisioners creates a separate communicator instance. Resources with multiple provisioner blocks leaked multiple connections, exacerbating socket exhaustion.

## 3. **Solution: Protocol-First Cleanup Order**
The fix calls `c.client.Close()` *before* `c.conn.Close()`, ensuring proper SSH shutdown happens at the protocol level before releasing file descriptors. The raw socket close is retained as defensive cleanup, establishing a clear teardown sequence: cancel goroutine → SSH disconnect → TCP close.

> id: f8dff57d-cc81-45aa-8b63-d3c4d747b85a

# Key Insights: SSH Communicator Connection Management

## 1. **Nil-Check Pattern for Safe Resource Cleanup**
**Decision:** The code consistently sets `c.conn = nil` after closing connections (lines 135, 202, 331) rather than relying solely on Close() operations.
**Why it matters:** This prevents double-close errors and ensures subsequent nil-checks catch stale references. The pattern is critical for reconnection logic where old connections must be explicitly nullified before establishing new ones.

## 2. **Race Condition Prevention Through Locking and Local References**
**Problem:** Long-running keepalive goroutines could race with connection reconnects.
**Solution:** The code uses sync.Mutex on the Communicator struct and creates local copies of the ssh client pointer (line 262: `sshClient := c.client`) before passing into goroutines. This prevents goroutines from accessing reallocated pointers during reconnection.

## 3. **Graceful Connection Liveness Detection**
**Decision:** A two-layer keepalive mechanism (periodic SendRequest + response timeout) instead of simple ping/pong.
**Why it matters:** Detects both dead connections (no response) and stuck connections (response timeout). The goroutine can independently close `sshConn` (lines 293, 299) when liveness fails, preventing indefinite hangs during long-running provisioning operations.

> id: 656694a5-3a8f-48ee-a014-07e259626a14

Hey! Bob here, ready to coordinate the team.

**Building:** Fix for Terraform SSH connections remaining ESTABLISHED after remote-exec provisioners complete

Let me analyze this issue. This is a bug in Terraform's SSH communicator — it's not a fix to the `lth` codebase, so rather than spinning up the full team workflow against the wrong repo, I'll research the root cause and produce the patch directly.

> id: be3d2efd-ffc1-4cb1-a7c3-a427183a3d11

Now restart the daemon with the new binary so all features activate.

> id: 9ea3705b-81da-4d59-ba63-cc39952c4999

Good. The root cause is clear: `Disconnect()` in the SSH communicator cancels the keepalive goroutine and closes the raw `net.Conn`, but **never calls `c.client.Close()`**. Without a proper SSH protocol-level disconnect, the OS TCP stack never sends FIN — the socket stays ESTABLISHED.

Let me produce the exact patch:

> id: fdfce8eb-337d-4a2a-881b-d8f3a294e6a9


## Memory IDs (for exploration)
Use these IDs to explore further:
  lth get <id>                    — read full memory
  lth graph show --from <id>      — traverse graph edges
  lth graph ppr --seeds <id,...>  — personalized pagerank from seeds

  8e3b88bc-8a6c-4b2d-9eb2-17fa12d891de
  eaa02943-9b77-4f28-92fd-3d0e31db92ef
  cee9f7b5-6143-454d-8751-207ba857b06c
  c121b38e-136d-4758-bf98-5c481e17d6d3
  9d1de0d1-e4eb-47fd-81ea-36c226e77985
  64b304cc-af31-41d6-97bb-14341314e6e2
  072ffffa-aab2-48c1-ab22-f2cbe4d83f9a
  88c2677e-efa5-4ad0-9ce3-58f257f0cce6
  61a1b905-a455-4187-a66d-815ef67d7644
  2a026a53-856e-438f-bd25-6f9bb87f6aca
  7110fd81-9ad3-4c5d-8ce9-0eb7aa1c4f92
  60f0d84f-49c9-45e6-87ef-a70640c67b03
  58212840-119f-4723-8068-7b0c1c652ce6
  76df40f7-c558-4492-abb4-5ea3a433ccc9
  986fcfb4-1518-479d-b5cb-235419cda00c
  f8dff57d-cc81-45aa-8b63-d3c4d747b85a
  656694a5-3a8f-48ee-a014-07e259626a14
  be3d2efd-ffc1-4cb1-a7c3-a427183a3d11
  9ea3705b-81da-4d59-ba63-cc39952c4999
  fdfce8eb-337d-4a2a-881b-d8f3a294e6a9
