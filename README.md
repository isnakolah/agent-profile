# agent-profile

`agent-profile` manages named, isolated Codex and Claude runtimes on macOS and Linux. Each profile owns separate `CODEX_HOME` and `CLAUDE_CONFIG_DIR` roots. Auth files are never copied, symlinked, or stored in the profile registry.

## Install

```sh
GOBIN="$HOME/.local/bin" go install github.com/isnakolah/agent-profile/cmd/agent-profile@latest
agent-profile doctor --json
agent-profile profile create work
agent-profile launcher install work --apply
```

Generated launchers are `work-codex` and `work-claude`. Login always runs the provider in its isolated root:

```sh
agent-profile login work codex
agent-profile login work claude
agent-profile snapshot work --json
agent-profile dashboard --watch
```

Usage data remains truthful: unavailable provider data is reported as unavailable, never guessed. Registry and snapshots are local disposable metadata; GitHub Project #18 is delivery authority.

Read [operations](docs/operations.md) and [architecture](docs/architecture.md). Active work lives in [Agent Profile Delivery](https://github.com/users/isnakolah/projects/18), not local task files.

License: MIT.
