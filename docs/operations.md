# Operations

## Profiles and authentication

Create profiles with `agent-profile profile create NAME`. This creates private provider roots and installs `NAME`, `NAME-codex`, and `NAME-claude` in `~/.local/bin`. Unrelated commands and symlinks are never overwritten. Existing generated legacy launchers can be adopted when their full content matches the old generator.

```sh
agent-profile auth work codex login
agent-profile auth work claude login
agent-profile auth work codex key
agent-profile auth work codex clear
```

`key` reads a masked terminal prompt or stdin. Do not put keys in command arguments or shell history. `clear` removes the manager-owned API key and selects provider login mode; it does not log out the provider. Existing sessions retain the credentials with which they started. Stop and relaunch them after switching authentication.

Provider config files and project settings still belong to the provider. Changing provider model, tool permissions, hooks or approval policies is done through that provider's normal configuration, not the profile registry.

## Sessions

`work codex` lists sessions for work/Codex/current directory and offers a new session. `work codex --model MODEL` starts a new invocation. Arguments are never split, quoted again, or evaluated as shell text. Noninteractive commands preserve stdin, stdout, stderr and exit codes.

Session states are `running`, `exited` or `interrupted`; running sessions also show the controller and number of viewers. Detach with Ctrl-] then d. Take control with Ctrl-] then t. Set `AGENT_PROFILE_DETACH_KEY=ctrl-b` to change the prefix. Use a viewer terminal at least as large as the controller's terminal.

```sh
agent-profile session list --json
agent-profile session attach SESSION_ID
agent-profile session attach SESSION_ID --view
agent-profile session stop SESSION_ID
agent-profile session remove SESSION_ID
```

Stopping sends SIGTERM to the process group, followed by SIGKILL after three seconds if it has not exited. Removing a list entry requires that its process has stopped and leaves provider conversation history intact. A profile cannot be removed while it owns running sessions.

## Background status refresh

```sh
agent-profile service install          # inspect generated definition
agent-profile service install --apply
agent-profile service enable --apply
agent-profile service status
agent-profile service disable --apply
```

macOS uses a launchd user agent; Linux uses a systemd user timer. Refresh runs every five minutes. Service startup captures the executable path, state root and PATH so it can find the provider CLIs. Reinstall the definition after moving the executable. The refresh service is independent of session workers and does not automatically restart conversations.

The dashboard reads cached status asynchronously and refreshes its session list every two seconds. Use Profile settings → Refresh provider status for a fresh authentication probe. Provider API-key status says “configured (not verified)”; unavailable quota data is never fabricated. `agent-profile notify NAME MESSAGE` sends an explicit native notification and deduplicates successful deliveries for an hour. Notification content is not written to the event log.

## Recovery

- **Reboot or missing worker:** choose the interrupted entry, then the provider's resume picker. No prompt is replayed. If the working directory was removed, restore it or start a new session in an existing directory.
- **Locked credential vault:** unlock Keychain/Secret Service and retry. On headless Linux, run a Secret Service implementation in the user's D-Bus session for API-key support.
- **Expired provider login:** run the profile's `auth ... login` command and create a new session. Usage and auth are reported separately.
- **Registry migration:** schema 1 is backed up to `profiles.json.v1.bak`. For rollback, stop manager commands and sessions, restore that backup and use the prior executable. Never restore someone else's auth files.
- **Unavailable provider:** install its official CLI and ensure it is on PATH; run `doctor --json`.
- **Protocol mismatch after upgrade:** old workers retain their original executable. Reattach with the matching version or stop and recover through the provider picker. Session metadata remains readable.

## Uninstall

Stop the sessions you intend to terminate. Run `agent-profile service uninstall --apply` to remove only the refresh service. `profile remove NAME` deletes that profile's runtime roots, manager-owned API keys and managed launchers; this is an explicit destructive operation. Back up provider conversations if you want them retained. Finally remove the `agent-profile` executable. Removing the executable alone does not stop existing workers.
