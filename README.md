# Agent Profile

**Your accounts. Your sessions. One place.**

Run the original Codex and Claude interfaces with separate work and personal credentials. Close a terminal, come back later, and pick up the same running session—without tmux or Zellij.

```sh
work codex --model MODEL
personal claude
```

## Get started

Requires macOS or Linux, Go 1.26+, and the provider CLIs you want to use. macOS builds use the system C toolchain for Keychain access; Linux API keys require an unlocked Secret Service vault.

```sh
GOBIN="$HOME/.local/bin" go install github.com/isnakolah/agent-profile/cmd/agent-profile@latest
# Ensure ~/.local/bin is on PATH.
agent-profile profile create work
agent-profile profile create personal
agent-profile auth work codex login
agent-profile auth personal claude login
work codex
```

Run `agent-profile` for the dashboard, or `work` to see one profile. If sessions already exist in the current project, `work codex` offers a session picker. Extra provider arguments start a new invocation and pass through unchanged. Help, management commands, print mode, and piped commands retain ordinary shell behavior.

## Leave and return

| Action | How |
| --- | --- |
| Detach while the provider keeps running | **Ctrl-]**, then **d**, or close the terminal |
| Find sessions | `agent-profile session list` or the dashboard |
| Watch another terminal's session | Select **View only**, or `session attach ID --view` |
| Transfer keyboard control | Select **Take control**, or **Ctrl-]**, then **t** |
| Send a literal prefix | Press **Ctrl-]** twice |
| Stop a provider | `agent-profile session stop ID` |
| Remove an old list entry | `agent-profile session remove ID` |

There is one keyboard controller and any number of read-only viewers. The controller determines terminal dimensions; use a viewer terminal at least that size. Set `AGENT_PROFILE_DETACH_KEY=ctrl-b` to choose another control-key prefix.

After reboot, interrupted sessions remain in the list. Selecting one opens the provider's own conversation-resume picker. Original prompts are never replayed automatically, and provider processes do not run while the computer is off or asleep.

## Credentials

Use separate provider logins, or set an API key through a masked prompt:

```sh
agent-profile auth work codex key
agent-profile auth work claude key
```

API keys live in macOS Keychain or Linux Secret Service. The selected key enters only its provider process environment. Profile metadata, launchers, logs, and session records contain no key values. Provider login credentials remain in their profile-specific provider storage. A locked vault requires unlocking; there is no plaintext fallback for manager-owned API keys.

## Learn more

- [Operations and recovery](docs/operations.md)
- [Source installation and verification](docs/source-install.md)
- [Architecture and contributor guide](docs/architecture.md)

Runtime state is local. Delivery work and evidence live in [Agent Profile Delivery](https://github.com/users/isnakolah/projects/18). See [release verification](docs/release-verification.md) for the distinction between automated checks and human/provider acceptance.

MIT licensed.
