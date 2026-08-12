# Source installation

Install on macOS or Linux with Go 1.26:

```sh
GOBIN="$HOME/.local/bin" go install github.com/isnakolah/agent-profile/cmd/agent-profile@latest
agent-profile doctor --json
```

Create a profile, then run provider login from its isolated launcher. Usage remains
explicitly unavailable when a provider CLI does not expose quota data. Service
installation writes launchd or systemd user definitions; enabling those services is
an operator action on each host.
