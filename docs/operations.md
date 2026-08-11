# Operations

Create profile, then generate launchers and authenticate each provider separately:

```sh
agent-profile profile create work
agent-profile launcher install work --apply
agent-profile login work codex
agent-profile login work claude
agent-profile refresh work
agent-profile doctor --json
```

`CODEX_HOME` and `CLAUDE_CONFIG_DIR` must point to profile-specific directories. Never switch profiles by copying or symlinking provider auth. Delete a profile only with explicit `agent-profile profile remove NAME`.

Install host refresh service with `agent-profile service install --apply`; review generated launchd/systemd files before loading them. Notifications depend on native host support. Provider usage may remain unavailable when provider APIs or CLI output do not expose it.

For recovery, reinstall from source, recreate registry metadata, and authenticate each profile again. Do not restore credentials from shell history, source control, Project comments, or Wiki pages.
