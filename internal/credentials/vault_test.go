package credentials

import (
	"os"
	"testing"
)

func TestVaultRoundTrip(t *testing.T) {
	if os.Getenv("AP_VAULT_INTEGRATION") != "1" {
		t.Skip("set AP_VAULT_INTEGRATION=1 with an unlocked OS vault")
	}
	root := t.TempDir()
	const value = "agent-profile-disposable-test-value"
	if e := Set(root, "test", "codex", value); e != nil {
		t.Fatal(e)
	}
	defer Delete(root, "test", "codex")
	got, e := Get(root, "test", "codex")
	if e != nil || got != value {
		t.Fatal("vault read failed")
	}
	if e = Set(root, "test", "codex", "replacement"); e != nil {
		t.Fatal(e)
	}
	got, e = Get(root, "test", "codex")
	if e != nil || got != "replacement" {
		t.Fatal("vault replacement failed")
	}
	if e = Delete(root, "test", "codex"); e != nil {
		t.Fatal(e)
	}
	if _, e = Get(root, "test", "codex"); e == nil {
		t.Fatal("deleted credential was readable")
	}
}
