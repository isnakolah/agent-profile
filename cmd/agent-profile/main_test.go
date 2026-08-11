package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidProfileNames(t *testing.T) {
	for _, name := range []string{"work", "claude-main", "a_1", "team.dev"} {
		if !validName(name) {
			t.Errorf("validName(%q) = false", name)
		}
	}
	for _, name := range []string{"", "../../escape", "with space", "ümlaut"} {
		if validName(name) {
			t.Errorf("validName(%q) = true", name)
		}
	}
}

func TestLauncherScriptIsolated(t *testing.T) {
	script := launcherScript("codex", "/private/profile/codex")
	if want := `export CODEX_HOME="/private/profile/codex"`; !contains(script, want) {
		t.Fatalf("launcher missing isolated environment: %s", script)
	}
	if contains(script, "cp ") || contains(script, "ln -s") {
		t.Fatal("launcher must not copy or link live auth state")
	}
}

func TestClaudeLauncherUsesOnlyClaudeConfigDir(t *testing.T) {
	script := launcherScript("claude", "/profiles/demo/claude")
	if !contains(script, `export CLAUDE_CONFIG_DIR="/profiles/demo/claude"`) {
		t.Fatalf("Claude launcher missing isolated config root: %s", script)
	}
	if contains(script, "CODEX_HOME") || contains(script, "HOME=") {
		t.Fatalf("Claude launcher changed unrelated runtime homes: %s", script)
	}
}

func TestDoctorReportDoesNotExposeCredentials(t *testing.T) {
	r := DoctorReport{Version: version, OS: "test", Root: "/private/config", Profiles: 2, Codex: "unavailable", Claude: "unavailable"}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(b), "token") || contains(string(b), "secret") || contains(string(b), "password") {
		t.Fatalf("doctor report exposes credential-like data: %s", b)
	}
}

func TestProviderLoginSpecs(t *testing.T) {
	command, args, err := providerLoginSpec("codex")
	if err != nil || command != "codex" || len(args) != 1 || args[0] != "login" {
		t.Fatalf("Codex login spec = %q %v %v", command, args, err)
	}
	command, args, err = providerLoginSpec("claude")
	if err != nil || command != "claude" || len(args) != 2 || args[0] != "auth" || args[1] != "login" {
		t.Fatalf("Claude login spec = %q %v %v", command, args, err)
	}
	if _, _, err := providerLoginSpec("unknown"); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func TestAtomicJSONAndRegistryMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")
	p := Profile{Name: "demo", CodexHome: filepath.Join(dir, "codex"), ClaudeConfigDir: filepath.Join(dir, "claude")}
	if err := atomicJSON(path, Registry{SchemaVersion: 1, Profiles: []Profile{p}}, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("registry mode = %o", info.Mode().Perm())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Registry
	if err := json.Unmarshal(b, &got); err != nil || len(got.Profiles) != 1 {
		t.Fatalf("registry unreadable: %v", err)
	}
}

func TestProfileLifecycleKeepsProviderRootsSeparate(t *testing.T) {
	root := t.TempDir()
	s := &store{root: root, registryPath: filepath.Join(root, "profiles.json"), eventsDir: filepath.Join(root, "snapshots"), registry: Registry{SchemaVersion: 1}}
	if err := os.MkdirAll(s.eventsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.create("demo"); err != nil {
		t.Fatal(err)
	}
	p, err := s.profile("demo")
	if err != nil {
		t.Fatal(err)
	}
	if p.CodexHome == p.ClaudeConfigDir {
		t.Fatal("provider roots must be distinct")
	}
	if _, err := os.Stat(p.CodexHome); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ClaudeConfigDir); err != nil {
		t.Fatal(err)
	}
	if err := s.remove("demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(p.CodexHome)); !os.IsNotExist(err) {
		t.Fatalf("profile roots remain after remove: %v", err)
	}
}

func contains(s, want string) bool { return len(s) >= len(want) && (s == want || index(s, want) >= 0) }

func index(s, want string) int {
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return i
		}
	}
	return -1
}
