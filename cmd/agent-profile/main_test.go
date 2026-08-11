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

func contains(s, want string) bool { return len(s) >= len(want) && (s == want || index(s, want) >= 0) }

func index(s, want string) int {
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return i
		}
	}
	return -1
}
