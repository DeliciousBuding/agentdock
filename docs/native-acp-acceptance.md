# Native ACP Acceptance

This runbook validates AgentDock's built-in ACP client against both a fake protocol agent and real host-installed adapters without embedding adapter credentials or machine-specific paths in the repository.

## Acceptance boundary

The runtime under test is the AgentDock process, its built-in `acp_session`, `acp_prompt`, and `acp_interaction` MCP tools, and one locally configured ACP adapter process.

Use separate AgentDock service instances when validating Claude and Codex simultaneously. Each instance must have its own:

- service account or equivalent OS security boundary;
- `AGENTDOCK_HOME`;
- loopback listen port;
- AgentDock authentication secret;
- ACP adapter command and environment mapping;
- allowed workspace root;
- external hostname or tunnel route, when remote MCP testing is required.

Do not share session stores between adapters.

`AGENTDOCK_HOME` and `AGENTDOCK_DEFAULT_DIR` are read directly by AgentDock and must be set to absolute, instance-specific paths. They are not child-adapter environment mappings; omitting them causes the process to use the current user's default AgentDock state and workspace directories.

## Secret handling

1. Store provider credentials only in a service-manager environment file or equivalent secret store with owner-only permissions.
2. Pass credentials to the adapter through `AGENTDOCK_ACP_ENV_FROM_ENV_JSON`.
3. Never place credentials in Git, `AGENTDOCK_ACP_ARGS_JSON`, a systemd `ExecStart`, shell history, process arguments, test evidence, screenshots, or PR text.
4. Use a dedicated, revocable test credential with a narrow budget and rotate it after acceptance.
5. Capture only variable names, file modes, hashes, statuses, and redacted diagnostics as evidence.

## Adapter installation

Install a pinned adapter version outside the AgentDock repository. Do not use `npx` or another online package runner in the service definition.

### Claude

Use the published `@agentclientprotocol/claude-agent-acp` package. Configure AgentDock with the absolute Node executable and absolute installed adapter entry point.

The adapter is backed by the Claude Agent SDK. Provider-specific variables remain host configuration and are not part of the AgentDock contract.

### Codex

Use the published `@agentclientprotocol/codex-acp` package. Configure AgentDock with its absolute installed executable or absolute Node entry point.

The adapter starts the Codex App Server and maps Codex events and approvals into ACP. A separately installed Codex CLI can be selected with the adapter's documented host environment when testing a specific CLI build.

## Generic service configuration

The following is a template. Replace placeholders only in the protected service environment, not in repository files:

```dotenv
AGENTDOCK_HOME=/var/lib/agentdock-acp-<adapter>
AGENTDOCK_DEFAULT_DIR=/srv/agentdock-acp-workspaces/<adapter>
AGENTDOCK_HOST=127.0.0.1
AGENTDOCK_PORT=<loopback-port>
AGENTDOCK_AUTH_TOKEN=<service-secret>

AGENTDOCK_ACP_ENABLED=true
AGENTDOCK_ACP_AGENT=<stable-adapter-name>
# Option A: Node plus an installed JavaScript adapter entry point.
AGENTDOCK_ACP_COMMAND=/absolute/path/to/node
AGENTDOCK_ACP_ARGS_JSON=["/absolute/path/to/adapter-entry.js"]
# Option B: a directly executable adapter. Do not also pass its entry point.
# AGENTDOCK_ACP_COMMAND=/absolute/path/to/adapter
# AGENTDOCK_ACP_ARGS_JSON=[]
AGENTDOCK_ACP_ALLOWED_ROOTS=/srv/agentdock-acp-workspaces/<adapter>
AGENTDOCK_ACP_ENV_FROM_ENV_JSON={"CHILD_PROVIDER_KEY":"HOST_PROVIDER_KEY"}
AGENTDOCK_ACP_MAX_CONCURRENT_PROMPTS=2
AGENTDOCK_ACP_INTERACTION_TIMEOUT_MS=300000
```

The service must run without root privileges. The service account should own only its AgentDock home and test workspace. Allowed roots are not an OS sandbox; filesystem permissions, container mounts, service users, and network policy remain the real security boundary.

## Phase 1: static and protocol checks

Record the commit under test, adapter package name/version, adapter executable path hash, OS, architecture, Go version, Node version when relevant, and AgentDock configuration variable names.

Run the repository's normal format, test, race, vet, and build gates. Run the opt-in real-adapter initialize smoke only with a dedicated test environment.

Acceptance requires:

- ACP wire version `1` is negotiated;
- optional `agentInfo`, when present, is recorded exactly as advertised, and omission does not fail initialization;
- optional capabilities are recorded exactly as advertised;
- no adapter-name-based capability assumptions occur;
- the adapter process and descendants are reclaimed when AgentDock stops.

## Phase 2: MCP discovery

Through the authenticated MCP endpoint:

1. Confirm `server_info` reports ACP enabled, the expected local adapter name, and only the intended allowed root.
2. Confirm `acp_session`, `acp_prompt`, and `acp_interaction` are present.
3. Inspect all three schemas.
4. Confirm callers cannot provide adapter commands, arguments, environment mappings, provider credentials, or new allowed roots.
5. Call `acp_session` with `action=info` and save the redacted initialize result.

Do not continue if the endpoint is unauthenticated, binds directly to a public interface, or reports an unexpected root.

## Phase 3: session lifecycle

Use an empty disposable Git repository inside the allowed root.

1. `acp_session info`.
2. If authentication methods are advertised, select only the intended advertised method with `acp_session authenticate`.
3. `acp_session new` with the disposable repository as `cwd`.
4. `list` and `inspect`; confirm only AgentDock's opaque `acps_*` ID is needed by the caller.
5. Exercise `load`, `resume`, `fork`, `set_mode`, `set_config`, `close`, and `delete` only when the adapter advertises the required capability or option.
6. Verify unsupported optional operations return a capability error rather than being attempted optimistically.
7. Confirm a closed or deleted session cannot start another prompt.

## Phase 4: prompt and event stream

1. Start a harmless prompt that asks the agent to inspect the disposable repository and return a short summary without changing files.
2. Confirm `acp_prompt start` returns immediately with an `acpr_*` run ID.
3. Poll `events` with `after_seq=0`, then pass each returned `next_seq` unchanged into the next call. While `has_more=true`, drain the next page without long polling.
4. Confirm event sequence numbers are monotonic, `next_seq <= latest_seq`, and no retained event is skipped across pages.
5. Supply an `after_seq` newer than `latest_seq` in a disposable run and confirm `ACP_CURSOR_AHEAD` is returned rather than an empty poisoned cursor.
6. Confirm `first_seq`, `dropped_count`, and `truncated` make bounded-ring eviction explicit during a high-volume synthetic or staging-only run.
7. Inject a staging-only update larger than 512 KiB and confirm it remains valid JSON with `update_truncated=true`, `original_update_bytes`, and a bounded preview.
8. Confirm terminal status and terminal event agree in the same observation page.
9. Confirm stale wake-ups do not produce repeated immediate empty pages while a run is still active.
10. Confirm complete model transcripts and event streams are not written to AgentDock's session store.
11. Confirm `acp_session info.context_policy` reports the adapter as history owner and disables AgentDock transcript persistence and replay.

## Phase 5: permissions

Use a prompt that proposes one harmless, easily reversible file write inside the disposable repository.

1. Observe the `permission_request` event.
2. List and inspect the pending interaction.
3. Confirm every exposed `option_id` came from the adapter request.
4. Confirm options representing indefinite or always authorization are absent.
5. Select a one-time option and verify the adapter continues.
6. Repeat with `acp_interaction cancel` and verify the adapter receives the cancelled outcome.
7. Cancel a prompt while a permission request is pending and verify the interaction settles instead of remaining pending until timeout.
8. Verify a serialized `toolCall` larger than 256 KiB is cancelled without being retained.
9. Verify the ninth pending request for one session and the thirty-third pending request globally are cancelled without expanding the interaction map.
10. Confirm `acp_session info.interaction_policy` reports the same limits used by the runtime.

Never approve unrestricted shell or filesystem access merely to make the test pass.

## Phase 6: cancellation, steering, and concurrency

1. Start a deliberately waiting prompt and cancel it by `run_id`.
2. Confirm cancelling with a mismatched `session_id` and `run_id` is rejected.
3. Confirm the run emits a cancelled terminal event and releases its global concurrency slot.
4. If steering is advertised, steer a running prompt and verify either native `injected` behavior or an observable `startedNewTurn` result with a new run id.
5. For a host-owned steering fallback, verify the old run is cancelled, the replacement run completes, and the same remote session id/transcript is retained.
6. Steer an idle session whose adapter returns `promptRequired` and verify AgentDock starts a normal observable run rather than requiring client-side resubmission.
7. Exercise the configured global prompt limit across separate sessions.
8. Confirm a second concurrent prompt on the same session is rejected.
9. Re-read a cancelled run after a late adapter response and confirm its status and terminal event remain cancelled.

## Phase 7: recovery and terminal transitions

1. Start a harmless long-running prompt.
2. Restart AgentDock without replaying the prompt.
3. Confirm the persisted session becomes `interrupted` with `agentdock_restart`.
4. Explicitly load or resume the session and decide whether to continue.
5. Confirm AgentDock does not prepend previously observed event text to the continuation prompt and does not create a second transcript file.
6. In a disposable instance, race close/delete against prompt completion and verify late finalization cannot recreate deleted state or overwrite closed state.
7. Confirm pending interactions expire or cancel during shutdown.

## Phase 8: path and state hardening

Verify all failures are explicit and do not reach the adapter:

- relative or nonexistent allowed root in host configuration;
- filesystem root as an allowed root;
- session `cwd` outside allowed roots;
- `additional_directories` outside allowed roots;
- symlink escape from an allowed root;
- too many additional directories;
- malformed, oversized, schema-incompatible, ID-mismatched, or symlink session-state file in a disposable AgentDock home.

After state-hardening tests, restore or delete the disposable AgentDock home. Never corrupt a shared or production state directory.

## Phase 9: adapter-specific evidence

### Claude adapter

Capture evidence for:

- initialize identity and capabilities;
- streamed assistant and tool-call updates;
- a one-time permission response;
- cancellation;
- load or resume when advertised;
- steering behavior from the pinned adapter version;
- for `claude-agent-acp` through `0.64.2`, the cancel → remote close/load → replacement-run compatibility path and the replacement run id;
- clean process-tree termination.

### Codex adapter

Capture evidence for:

- initialize identity, auth methods, modes, and config options;
- Codex App Server-backed assistant, reasoning, plan, shell/file-change, and approval events that the selected task naturally produces;
- a one-time approval response;
- cancellation and mode change;
- a remote retry/error case proving that a failed App Server turn cannot be reported as a completed AgentDock run;
- a retry-then-success case proving the compatibility guard does not create false failures;
- subagent or review metadata only when naturally supported by the installed adapter version;
- clean process-tree termination.

Do not claim a feature merely because the adapter README lists it. Record only behavior observed from the pinned version under test.

## Remote MCP acceptance

When another ChatGPT instance performs the test:

1. Put AgentDock behind the existing authenticated reverse tunnel or access layer.
2. Route the hostname only to the instance's loopback port.
3. Keep direct public origin access closed.
4. Give the tester only the MCP endpoint and its AgentDock authentication flow, never the provider credential or service environment file.
5. Require the tester to use only the disposable allowed-root repository.
6. Revoke temporary endpoint credentials after acceptance.

## Evidence bundle

The handoff should contain:

- commit and adapter versions;
- instance names and loopback ports, with private infrastructure details kept in the private operations repository;
- service status and file ownership/mode checks;
- redacted `server_info` and `acp_session info` outputs;
- session/run/interaction IDs created during the test;
- event-type timeline without model secrets or full prompt content;
- permission outcomes;
- restart, cancellation, path-denial, and process-cleanup evidence;
- known adapter-version gaps;
- rollback commands and cleanup confirmation.

## Rollback

1. Disable the external tunnel route.
2. Stop and disable the staging AgentDock service instance.
3. Confirm the adapter process tree is gone.
4. Revoke the AgentDock endpoint credential and provider test credential.
5. Archive only redacted evidence.
6. Remove the disposable workspace and AgentDock home.
7. Leave unrelated AgentDock instances, reverse tunnels, CI runners, and production services untouched.
