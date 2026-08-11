# Architecture

The command is a small standard-library core with one Bubble Tea dashboard boundary.

- Registry: metadata-only JSON under the platform user config directory, written atomically with mode `0600`.
- Runtime isolation: each profile gets independent Codex and Claude directories with mode `0700`.
- Providers: adapters execute installed provider CLIs with only the matching environment variable. Missing or unsupported usage data stays `unavailable`.
- Refresh: snapshots are written atomically under the profile cache and include checked time, status, version, and error.
- Launchers: generated scripts set one isolated provider root and `exec` the provider; they never copy auth.
- Services: launchd user plist on macOS and systemd user unit on Linux.
- Notifications: native `osascript` or `notify-send`, invoked only by explicit operator/alert paths.

GitHub Project state, Issues, evidence, and the generated Wiki journal remain canonical for delivery. Runtime files are disposable and never a delivery ledger.
