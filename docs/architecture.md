# Architecture

The executable dispatches into `internal/app`. Each application operation is shared by the CLI and terminal dashboard. The provider's interactive UI runs as the provider executable, not a reimplementation of its chat interface.

## Boundaries

| Package | Responsibility |
| --- | --- |
| `app` | Commands, profile lifecycle, cached refresh, notifications, host services |
| `provider` | Provider invocation, auth environment isolation, argument passthrough |
| `credentials` | macOS Security framework and Linux Secret Service adapters |
| `state` | Private directories, process locks, atomic JSON writes |
| `session` | Independent PTY workers, protocol, screen state and terminal clients |
| `tui` | Metadata-only navigation, input and responsive dashboard rendering |

Keep provider output out of the dashboard data model. Keep secrets out of persistent structs. Tests inject controlled provider processes instead of invoking a model.

## Native session lifecycle

A launch spawns a detached copy of `agent-profile _session-worker`. The parent sends the launch specification over a private Unix socket. The worker creates the PTY and provider process; subsequent clients attach to the worker directly. The command's process, arguments, directory and environment are not interpreted by a shell.

Workers are independent of the dashboard and status-refresh service. There is deliberately no central PTY-owning coordinator whose restart would kill all sessions. Listing scans durable metadata and asks live workers for attachment status. A missing worker changes a previously running entry to `interrupted` in the returned view. Exited workers record an exit code.

The local protocol uses a four-byte big-endian length followed by versioned JSON, with a 4 MiB frame limit. Terminal bytes are base64-encoded by JSON. Sockets reside in a private per-user directory under `/tmp`, keeping paths within Darwin's Unix-socket limit. Frames are never logged. API keys and original prompt arguments are transferred only in the initial in-memory launch specification.

Each worker maintains a bounded virtual terminal and 1,000 lines of in-memory scrollback. Reattachment reconstructs the screen and modes before live output continues. Per-client output queues are bounded; slow clients disconnect rather than block provider progress. One controller supplies input and resize events; viewers' input and resize requests are ignored until explicit takeover.

On reboot or worker loss, the UI offers the provider's native resume picker in the original directory. No unreliable “latest conversation” matching and no original-prompt replay are used. Conversation IDs are not inferred from terminal output.

## Persistence and authentication

Registry schema 2 adds per-provider authentication mode metadata. Loading schema 1 validates it, writes a private `profiles.json.v1.bak`, and atomically migrates. Unknown schemas, duplicate names, unsafe roots and profile symlinks fail closed. Mutations hold an advisory process lock; refresh releases it while probing providers and merges results under a fresh lock.

Profile roots have mode `0700`; JSON has mode `0600`. Auth environments remove inherited provider credentials before setting the selected provider root and optional API key. The manager does not copy provider auth files. Codex API-key mode uses a named OpenAI provider configured with an environment-key reference; no key is written to Codex configuration or its auth file. Provider arguments follow the manager's authentication configuration unchanged.

The macOS vault uses the Security framework through cgo. Linux uses an encrypted Secret Service D-Bus session. A build without cgo on macOS reports Keychain unavailable rather than substituting a file vault.

## Development

```sh
go test -race -timeout 120s ./...
go vet ./...
go build ./cmd/agent-profile
```

`AGENT_PROFILE_HOME` selects an isolated state root for development. `AP_VAULT_INTEGRATION=1 go test ./internal/credentials` performs a disposable vault round trip. It never contacts a model provider.

Follow `.agent/rules/github-first-delivery.md`: changes belong to a claimed leaf issue; PR evidence and the generated Wiki journal remain canonical delivery records.
