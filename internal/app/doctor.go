package app

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
)

func doctorCommand(args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.close()
	r := DoctorReport{Version: version, OS: runtime.GOOS, Root: store.root, Profiles: len(store.registry.Profiles), Codex: executableStatus("codex"), Claude: executableStatus("claude")}
	if len(args) > 0 && args[0] == "--json" {
		return printJSON(r)
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(b))
	return nil
}

func executableStatus(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return "unavailable"
}
