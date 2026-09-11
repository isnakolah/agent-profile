# Source installation

Go 1.26+, macOS or Linux, and the chosen provider CLIs are required. macOS needs the Command Line Tools C compiler for native Keychain support. Linux API-key profiles need a Secret Service implementation in the user's D-Bus session.

From a checkout:

```sh
go test -race -timeout 120s ./...
go vet ./...
GOBIN="$HOME/.local/bin" go install ./cmd/agent-profile
agent-profile doctor --json
```

From a published source tag, replace `VERSION` with that tag:

```sh
GOBIN="$HOME/.local/bin" go install github.com/isnakolah/agent-profile/cmd/agent-profile@VERSION
```

Ensure `~/.local/bin` is on PATH. Create profiles, authenticate each provider separately, and install/enable status refresh as described in [operations](operations.md).

The CI matrix runs tests, race detection, vet, build and isolated source-install smoke checks on macOS and Linux. The source-install workflow also runs on version tags. These checks do not establish live account isolation or real reboot acceptance; see [release verification](release-verification.md).
