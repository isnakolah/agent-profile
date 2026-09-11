package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentAtomicWritersLeaveCompletePrivateJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := JSON(path, map[string]int{"value": i}); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var v map[string]int
	if e = json.Unmarshal(b, &v); e != nil {
		t.Fatal(e)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("nonprivate file")
	}
	files, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".write-*"))
	if len(files) != 0 {
		t.Fatal("temporary files leaked")
	}
}
func TestLockRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	_ = os.WriteFile(target, []byte("preserve"), 0600)
	path := filepath.Join(dir, "lock")
	_ = os.Symlink(target, path)
	if f, e := Lock(path); e == nil {
		Unlock(f)
		t.Fatal("accepted symlink lock")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "preserve" {
		t.Fatal("target modified")
	}
}
