package app

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRejectDotNamesAndSymlinkProfile(t *testing.T) {
	for _, name := range []string{".", "..", "-flag"} {
		if validName(name) {
			t.Errorf("accepted %q", name)
		}
	}
	root := t.TempDir()
	outside := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "profiles"), 0700)
	_ = os.Symlink(outside, filepath.Join(root, "profiles", "work"))
	if e := safeProfilePath(root, "work"); e == nil {
		t.Fatal("accepted profile symlink")
	}
}
func TestMigrationAndConcurrentCreates(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENT_PROFILE_HOME", root)
	_ = os.WriteFile(filepath.Join(root, "profiles.json"), []byte(`{"schema_version":1,"profiles":[]}`), 0600)
	s, e := openStore()
	if e != nil {
		t.Fatal(e)
	}
	if s.registry.SchemaVersion != 2 {
		t.Fatal("not migrated")
	}
	s.close()
	if _, e = os.Stat(filepath.Join(root, "profiles.json.v1.bak")); e != nil {
		t.Fatal("missing migration backup")
	}
	var wg sync.WaitGroup
	for _, name := range []string{"one", "two", "three", "four"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, e := openStore()
			if e != nil {
				t.Error(e)
				return
			}
			defer s.close()
			if e = s.create(name); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	s, e = openStore()
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	if len(s.registry.Profiles) != 4 {
		t.Fatal("concurrent create lost profiles")
	}
}
func TestLauncherTrailingApplyAndCollision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AGENT_PROFILE_HOME", filepath.Join(home, "data"))
	s, e := openStore()
	if e != nil {
		t.Fatal(e)
	}
	e = s.create("test-ap-profile")
	s.close()
	if e != nil {
		t.Fatal(e)
	}
	if e = launcherCommand([]string{"install", "test-ap-profile", "--apply"}); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(home, ".local", "bin", "test-ap-profile")
	b, e := os.ReadFile(path)
	if e != nil || !strings.Contains(string(b), launcherMarker) {
		t.Fatalf("launcher not installed: %v", e)
	}
	_ = os.WriteFile(path, []byte("unrelated"), 0755)
	if e = launcherCommand([]string{"install", "test-ap-profile", "--apply"}); e == nil {
		t.Fatal("overwrote foreign command")
	}
	b, _ = os.ReadFile(path)
	if string(b) != "unrelated" {
		t.Fatal("foreign launcher changed")
	}
}
func TestShellQuoteProtectsSpecialPaths(t *testing.T) {
	value := "space ' quote $HOME `touch nope` $(touch nope)"
	out, e := exec.Command("sh", "-c", "printf '%s' "+shellQuote(value)).Output()
	if e != nil || string(out) != value {
		t.Fatalf("shell quoting failed: %q %v", out, e)
	}
}
func TestUnknownRegistryVersionPreserved(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENT_PROFILE_HOME", root)
	path := filepath.Join(root, "profiles.json")
	b, _ := json.Marshal(Registry{SchemaVersion: 99})
	_ = os.WriteFile(path, b, 0600)
	if s, e := openStore(); e == nil {
		s.close()
		t.Fatal("accepted future schema")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(b) {
		t.Fatal("rewrote unknown schema")
	}
}
