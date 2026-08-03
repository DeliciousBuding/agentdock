# Native ACP Runtime

## Status

This document defines AgentDock's built-in Agent Client Protocol (ACP) client runtime.

The implementation targets stable ACP wire protocol version `1`. Optional methods are never assumed from the adapter name: AgentDock negotiates the protocol with `initialize` and uses the returned capabilities to decide whether `session/load`, additional workspace roots, `session/resume`, `session/fork`, `session/close`, `session/delete`, and steering are available.

ACP support is disabled by default and is configured only by the AgentDock host operator.

## Design goals

1. Treat ACP as a first-class AgentDock runtime, not as a dynamic MCP server.
2. Keep the public tool surface action-based and consistent with existing AgentDock tools.
3. Keep long agent turns outside the lifetime of one MCP tool request.
4. Preserve ACP's bidirectional protocol: session updates and agent-to-client requests share one managed JSON-RPC connection.
5. Own the adapter process tree through AgentDock's existing cross-platform process controller.
6. Restrict every ACP workspace path to host-configured allowed roots.
7. Persist only recovery metadata; do not duplicate full model transcripts or tool output on disk.
8. Keep the protocol core adapter-neutral. Claude and Codex are host-selected adapters, not protocol special cases.

## Non-goals

- AgentDock does not implement an ACP agent.
- AgentDock does not translate ACP into a dynamically registered MCP server.
- Remote MCP callers cannot change the ACP executable, arguments, environment mappings, or allowed roots.
- The first version does not advertise ACP client filesystem or terminal capabilities. An adapter therefore cannot ask AgentDock to perform arbitrary client-side filesystem or terminal operations through ACP.
- AgentDock does not automatically replay an interrupted prompt after restart.
- AgentDock does not persist complete ACP event streams or model transcripts.

## Architecture

```text
MCP client (ChatGPT, Claude, Codex, ...)
                    |
                    | AgentDock built-in MCP tools
                    v
+--------------------------------------------------+
| AgentDock Runtime                                |
|                                                  |
|  acp_session   acp_prompt   acp_interaction      |
|         \          |          /                 |
|          +---- internal/acp.Manager ----+        |
|               | sessions / runs        |        |
|               | event ring             |        |
|               | permission queue       |        |
|               | atomic session store   |        |
|               +-----------+------------+        |
|                           | NDJSON JSON-RPC      |
|                           v                     |
|                 managed ACP adapter process     |
+--------------------------------------------------+
```

The ACP manager is created and closed by the same `app.Runtime` that owns commands, dynamic MCP clients, Skills, tasks, and media services. There is no additional daemon, listener, port, or watchdog.

## Process lifecycle

The configured ACP command must be an absolute path.

- Windows: the adapter is attached to the existing AgentDock Job Object controller. Closing AgentDock terminates the whole adapter process tree.
- Linux and macOS: the adapter starts as the leader of a dedicated process group. Closing AgentDock terminates that process group.
- Unix-like platforms additionally require the configured command file to have an executable bit.
- Adapter stdout is reserved for ACP NDJSON messages. Adapter stderr is retained only as a bounded diagnostic tail; values explicitly inherited through the ACP environment mapping are redacted before diagnostics are returned.
- Remote JSON-RPC error messages and string diagnostic fields are also redacted and bounded before they cross the MCP tool boundary.
- AgentDock launches the adapter lazily on the first operation that needs a live connection.
- A dead adapter is replaced lazily. Session metadata remains local and is loaded or resumed only when requested.

The child environment starts from `envstore.MinimalSystemEnv()`. Platform-specific completion preserves the minimum variables required by normal tooling:

- Windows completes profile, application-data, temporary-directory, and Go module-cache variables when absent.
- Linux and macOS preserve `HOME`, `TMPDIR`, and explicitly inherited toolchain paths.
- Secrets are copied only through the host-controlled `AGENTDOCK_ACP_ENV_FROM_ENV_JSON` mapping.

## Protocol connection

`internal/acp.Connection` implements one bidirectional newline-delimited JSON-RPC 2.0 stream.

- Requests use monotonically increasing numeric IDs.
- Responses are routed through a pending-request map.
- Incoming notifications are dispatched independently from responses.
- Incoming agent requests are handled concurrently so a permission request cannot block the event reader.
- Context cancellation removes the pending request and emits `$/cancel_request`.
- Incoming `$/cancel_request` notifications cancel the matching agent-to-client request handler, including a permission request that became obsolete when a turn was cancelled.
- One JSON-RPC line is capped at 4 MiB.
- Writes are serialized.
- EOF and closed pipes are normal during explicit shutdown and do not make Runtime shutdown fail.

Initialization sends:

```json
{
  "protocolVersion": 1,
  "clientCapabilities": {},
  "clientInfo": {
    "name": "agentdock",
    "title": "AgentDock",
    "version": "<AgentDock version>"
  }
}
```

AgentDock rejects a negotiated wire version other than `1`.

## Host configuration

| Variable | Required | Meaning |
| --- | --- | --- |
| `AGENTDOCK_HOME` | for isolated instances | Absolute AgentDock state directory. Defaults to the current user's `.agentdock` directory. Use a distinct value for every concurrently running AgentDock instance. |
| `AGENTDOCK_DEFAULT_DIR` | for isolated instances | Absolute default workspace directory. Defaults to the current user's `AgentDock` directory. It should normally match the instance's primary ACP allowed root. |
| `AGENTDOCK_ACP_ENABLED` | no | Enables the built-in ACP runtime. Defaults to `false`. |
| `AGENTDOCK_ACP_AGENT` | no | Stable 1-64 character local profile name using letters, numbers, dot, underscore, or hyphen. Defaults to `claude`. |
| `AGENTDOCK_ACP_COMMAND` | when enabled | Absolute executable path. On Unix it must be executable. |
| `AGENTDOCK_ACP_ARGS_JSON` | no | JSON string array passed directly to the executable, without a shell. |
| `AGENTDOCK_ACP_ENV_FROM_ENV_JSON` | no | JSON object mapping child variable names to host variable names. Secret values are not stored in AgentDock state. |
| `AGENTDOCK_ACP_ALLOWED_ROOTS` | when enabled | Comma-separated absolute directories that may become ACP workspace roots. |
| `AGENTDOCK_ACP_MAX_CONCURRENT_PROMPTS` | no | Global prompt concurrency, from 1 to 8. Defaults to 2. |
| `AGENTDOCK_ACP_INTERACTION_TIMEOUT_MS` | no | Permission interaction timeout, from 1 second to 1 hour. Defaults to 300000 ms. |

Example using an absolute Node executable and an installed adapter entry point:

```bash
AGENTDOCK_HOME=/var/lib/agentdock-acp-claude
AGENTDOCK_DEFAULT_DIR=/srv/code/claude
AGENTDOCK_ACP_ENABLED=true
AGENTDOCK_ACP_AGENT=claude
AGENTDOCK_ACP_COMMAND=/absolute/path/to/node
AGENTDOCK_ACP_ARGS_JSON='["/absolute/path/to/claude-agent-acp/dist/index.js"]'
AGENTDOCK_ACP_ALLOWED_ROOTS=/srv/code,/srv/worktrees
AGENTDOCK_ACP_ENV_FROM_ENV_JSON='{"ANTHROPIC_API_KEY":"HOST_ANTHROPIC_API_KEY"}'
```

On Windows, `AGENTDOCK_ACP_ALLOWED_ROOTS` uses comma-separated Windows paths and the JSON arguments contain escaped backslashes when needed.

`AGENTDOCK_HOME` and `AGENTDOCK_DEFAULT_DIR` are first-class host configuration, not adapter environment mappings. They isolate AgentDock's own state and default workspace before the ACP manager is created. Running Claude and Codex at the same time therefore requires distinct values for both variables in addition to distinct ports and authentication credentials.

The command and arguments never pass through a shell. Do not configure `npx`, an online installer, or an unpinned package runner as the production executable.

### Claude and Codex adapters

AgentDock deliberately does not bundle or fork an adapter. Install and pin the adapter independently, then configure AgentDock with an absolute executable and absolute adapter entry point.

- Claude: [`agentclientprotocol/claude-agent-acp`](https://github.com/agentclientprotocol/claude-agent-acp) exposes the Claude Agent SDK through ACP.
- Codex: [`agentclientprotocol/codex-acp`](https://github.com/agentclientprotocol/codex-acp) starts the Codex App Server and maps Codex sessions, events, approvals, tools, plans, and subagents into ACP.

One AgentDock process owns one configured adapter. To expose Claude and Codex simultaneously, run two isolated AgentDock service instances with different homes, loopback ports, authentication credentials, adapter commands, and allowed roots. This keeps session stores and process ownership single-purpose and prevents cross-agent state ambiguity while reusing the same adapter-neutral runtime.

Never put API keys in `AGENTDOCK_ACP_ARGS_JSON`, a systemd `ExecStart`, shell history, or the process command line. Keep secrets in the service manager's protected environment file and map only the required names with `AGENTDOCK_ACP_ENV_FROM_ENV_JSON`.

## Workspace policy

All `cwd` and `additional_directories` values are resolved through native `filepath` semantics:

1. Relative paths resolve from the first configured allowed root.
2. Existing symlinks are evaluated.
3. The final path must be a directory.
4. `filepath.Rel` verifies that the final path remains under at least one allowed root.
5. Duplicate directories are removed with `os.SameFile`, preserving filesystem identity and platform case semantics.

The policy is enforced before the adapter receives a session lifecycle request. Additional directories are sent only when the agent advertises `sessionCapabilities.additionalDirectories`.

Allowed roots are a protocol policy, not an operating-system sandbox. The adapter and its child tools still run with the AgentDock service account's OS permissions. Operators should use a dedicated account, filesystem ACLs, container mounts, and network policy as the actual security boundary.

## Public tool surface

### `acp_session`

Action-based session lifecycle entry point:

- `info`: initialize the adapter and return negotiated identity and capabilities.
- `authenticate`: select one authentication method advertised by `initialize`. The public tool accepts only the advertised method ID and never accepts credential material.
- `new`: create and persist a session.
- `load`: load a persisted session when `loadSession` is advertised.
- `resume`: resume a persisted session when `sessionCapabilities.resume` is advertised.
- `fork`: create a new persisted session when `sessionCapabilities.fork` is advertised.
- `set_mode`: apply an agent-advertised mode.
- `set_config`: apply an agent-advertised string or boolean configuration option.
- `list`: list AgentDock's local persistent session records.
- `inspect`: inspect one local session record.
- `close`: cancel active work, call remote close when advertised, and mark the local session closed.
- `delete`: call remote delete when advertised, then remove the local session record.

AgentDock uses its own opaque `acps_*` identifier externally and keeps the adapter's session ID inside the private session record.

### `acp_prompt`

Action-based prompt execution entry point:

- `start`: validate and load the session, acquire a concurrency slot, start `session/prompt` in an AgentDock-owned background goroutine, and return an `acpr_*` run ID immediately.
- `events`: return events after `after_seq`; optional long polling is capped at 25 seconds.
- `steer`: use the adapter's advertised steering extension. An idle `promptRequired` outcome is converted into a standard, observable prompt run. For `@agentclientprotocol/claude-agent-acp` releases through `0.64.2`, whose injected steering can settle the owning prompt with an SDK EDE diagnostic, AgentDock uses a narrow compatibility path: cancel the old run, remotely close and reload the same transcript-backed session, then start a standard prompt and return its new `runId` together with the cancelled run id.
- `cancel`: emit `session/cancel` and cancel the local request context.

A session has at most one active prompt. The global limit is host-configurable.

Events have a monotonically increasing sequence and are stored in a bounded in-memory ring: at most 512 events or approximately 4 MiB per run, while always retaining the newest event. `next_seq` is the only supported continuation cursor and should be passed unchanged as the next `after_seq`. `latest_seq` identifies the newest event visible in the same snapshot, `has_more` tells the caller to drain another page immediately, and `first_seq`, `dropped_count`, and `truncated` make ring eviction explicit. A cursor newer than `latest_seq` is rejected with `ACP_CURSOR_AHEAD` instead of silently poisoning future reads.

One adapter update is capped at 512 KiB before it enters the event ring. Oversized updates remain valid JSON and retain their `sessionUpdate` type plus a bounded preview; the event exposes `update_truncated=true` and `original_update_bytes`. This prevents a single malformed or unexpectedly large adapter notification from defeating the run-level memory and MCP-response bounds.

Agent-provided `sessionUpdate` values are used as event types when available; terminal events are normalized to `completed`, `cancelled`, `interrupted`, or `error`. Run settlement and its terminal event are committed under one lock, so a caller cannot observe a settled status without the matching terminal event.

### `acp_interaction`

Action-based permission entry point:

- `list`: list pending or historical in-memory interactions.
- `inspect`: inspect one interaction.
- `respond`: select one currently offered permission option.
- `cancel`: return the ACP cancelled outcome.

The adapter's `session/request_permission` request remains open while the MCP client reviews the interaction. The reader continues processing notifications and other responses during this time.

AgentDock validates the session, interaction state, and exact option ID before responding. Options whose kind contains `always` are removed by the local policy, so a remote model cannot establish an indefinite approval through the public tool.

Permission interactions are memory-only and bounded independently from prompt events: at most 32 offered options, 256 KiB of serialized `toolCall` data, 32 pending interactions globally, and 8 pending interactions per session. An oversized request or a request beyond either pending limit is answered with the ACP cancelled outcome without retaining another interaction.

A selected permission is encoded exactly as:

```json
{
  "outcome": {
    "outcome": "selected",
    "optionId": "<offered option id>"
  }
}
```

Cancellation and timeout use:

```json
{
  "outcome": {
    "outcome": "cancelled"
  }
}
```

## Adapter compatibility normalization

Compatibility handling is gated by the exact initialized adapter identity and release boundary; configuration profile names never activate it.

- `@agentclientprotocol/codex-acp` can report a failed App Server turn as a normal ACP `end_turn` after emitting the remote failure text as an `agent_message_chunk`. AgentDock promotes this to `ACP_REMOTE_ERROR` only when a machine-readable retry error was observed and the final assistant chunk exactly matches that error. A retry followed by a different, normal assistant result remains completed.
- `@agentclientprotocol/claude-agent-acp` through `0.64.2` advertises injected steering but the bundled SDK can terminate the owning prompt with an EDE diagnostic. AgentDock avoids that broken injection path and returns an observable replacement run after a standard close/load lifecycle reset. Later versions automatically use native injected steering.

These guards do not patch adapter packages or inspect private transcript files. Unknown versions fail open to the negotiated protocol behavior instead of accumulating permanent vendor-specific assumptions.

## Persistent state and recovery

Session records live under:

```text
~/.agentdock/acp/sessions/<adapter_identity>/<acps_id>.json
```

`<adapter_identity>` is a stable hash of the configured adapter name. Each adapter therefore has an isolated recovery and remote-session namespace, while the configured name never becomes a filesystem path segment.

Records are written atomically with private permissions and contain only:

- local and remote session identifiers;
- profile name;
- canonical workspace roots;
- lifecycle status and stop reason;
- timestamps.

Full prompts, model responses, tool payloads, permission payload history, and event streams are not persisted. The configured adapter is the sole owner of transcript history. AgentDock does not concatenate event output into later prompts, does not maintain a second transcript, and never replays a prior prompt after restart. The `acp_session info` response reports this as `context_policy`, the bounded cursor contract as `event_policy`, permission bounds as `interaction_policy`, and observable steering behavior as `steering_policy`.

State loading is fail-closed. AgentDock rejects oversized, malformed, schema-incompatible, ID-mismatched, non-regular, or symlink session files rather than silently hiding them from recovery and session listings.

On startup, a persisted session left in `running` becomes `interrupted` with `agentdock_restart`. During graceful AgentDock shutdown, active runs become `interrupted` with `agentdock_shutdown`. Close and delete first seal the session against new operations, wait for already-started short lifecycle operations to finish, and then cancel active work. The terminal marker also prevents a late prompt finalizer from recreating a deleted session or overwriting a closed session. AgentDock never replays the prior prompt automatically; a caller must explicitly load or resume the session and decide what to do next.

## Tool annotations

AgentDock's `ToolSpec` single source of truth now supports MCP tool annotations. ACP entry points are conservatively marked as mutating, potentially destructive, and open-world because each action-based tool contains both read and write actions and the adapter may access external services.

Annotations improve client presentation but are not an authorization boundary. Runtime validation and host policy remain authoritative.

## Cross-platform verification

The ACP package is platform-independent except for existing process-control and small configuration/environment build-tag files.

Required PR verification:

```bash
go test ./...
go vet ./...
go build ./cmd/agentdock
```

Additionally:

- run the ACP fake-agent integration test, including a bidirectional permission request;
- run the ACP package under the Go race detector where supported;
- cross-compile `cmd/agentdock` for Linux amd64 and macOS arm64;
- compile ACP/config/envstore tests for Linux and macOS to catch build-tag drift;
- run an opt-in smoke initialization against the configured real adapter.

The complete adapter-neutral acceptance sequence is documented in [Native ACP Acceptance](native-acp-acceptance.md).

## Future extensions

Future changes may advertise narrowly scoped ACP client capabilities for filesystem, terminal, elicitation, or nested transcript rendering. Each capability must have its own host policy, path/size limits, tests, and explicit threat model before being enabled. They must not be inferred merely because a particular adapter supports them.
