package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/isnakolah/agent-profile/internal/tui"
)

const version = "0.1.0-dev"

type Profile struct {
	Name             string    `json:"name"`
	CreatedAt        time.Time `json:"created_at"`
	LastRefresh      time.Time `json:"last_refresh,omitempty"`
	CredentialReview string    `json:"credential_review,omitempty"`
	CodexHome        string    `json:"codex_home"`
	ClaudeConfigDir  string    `json:"claude_config_dir"`
}

type Registry struct {
	SchemaVersion int       `json:"schema_version"`
	Profiles      []Profile `json:"profiles"`
}

type ProviderSnapshot struct {
	Provider      string    `json:"provider"`
	Profile       string    `json:"profile"`
	Status        string    `json:"status"`
	Version       string    `json:"version,omitempty"`
	Authenticated string    `json:"authenticated"`
	Usage         string    `json:"usage"`
	Error         string    `json:"error,omitempty"`
	Checked       time.Time `json:"checked_at"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "agent-profile:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		usage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Println(version)
		return nil
	case "profile":
		return profileCommand(args[1:])
	case "launcher":
		return launcherCommand(args[1:])
	case "doctor":
		return doctorCommand(args[1:])
	case "snapshot":
		return snapshotCommand(args[1:])
	case "refresh":
		return refreshCommand(args[1:])
	case "login":
		return loginCommand(args[1:])
	case "dashboard":
		return dashboardCommand(args[1:])
	case "service":
		return serviceCommand(args[1:])
	case "notify":
		return notifyCommand(args[1:])
	default:
		return fmt.Errorf("unknown command %q; run agent-profile help", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `agent-profile — isolated Codex and Claude profile manager

Commands:
  profile create|list|show|remove NAME
  launcher install NAME [--apply]
  login NAME codex|claude
  snapshot NAME [--json]
  refresh [NAME|--all]
  dashboard [--watch]
  doctor [--json]
  service install [--apply]
  notify NAME MESSAGE

Profile state contains metadata only. Provider auth stays in isolated runtime roots.`)
}

func profileCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: profile create|list|show|remove NAME")
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	switch args[0] {
	case "create":
		if len(args) != 2 {
			return errors.New("usage: profile create NAME")
		}
		return store.create(args[1])
	case "list":
		return printJSON(store.registry.Profiles)
	case "show":
		if len(args) != 2 {
			return errors.New("usage: profile show NAME")
		}
		p, err := store.profile(args[1])
		if err != nil {
			return err
		}
		return printJSON(p)
	case "remove":
		if len(args) != 2 {
			return errors.New("usage: profile remove NAME")
		}
		return store.remove(args[1])
	default:
		return fmt.Errorf("unknown profile command %q", args[0])
	}
}

func launcherCommand(args []string) error {
	fs := flag.NewFlagSet("launcher install", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "write launchers")
	if len(args) < 2 || args[0] != "install" {
		return errors.New("usage: launcher install NAME [--apply]")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	p, err := store.profile(args[1])
	if err != nil {
		return err
	}
	bin, err := launcherDir()
	if err != nil {
		return err
	}
	for provider, root := range map[string]string{"codex": p.CodexHome, "claude": p.ClaudeConfigDir} {
		path := filepath.Join(bin, p.Name+"-"+provider)
		content := launcherScript(provider, root)
		if !*apply {
			fmt.Printf("%s\n%s\n", path, content)
			continue
		}
		if err := os.MkdirAll(bin, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			return err
		}
		fmt.Println(path)
	}
	return nil
}

func launcherScript(provider, root string) string {
	var env string
	var command string
	if provider == "codex" {
		env, command = "CODEX_HOME", "codex"
	} else {
		env, command = "CLAUDE_CONFIG_DIR", "claude"
	}
	return fmt.Sprintf("#!/bin/sh\nset -eu\nexport %s=%q\nexec %s \"$@\"\n", env, root, command)
}

func loginCommand(args []string) error {
	if len(args) != 2 || (args[1] != "codex" && args[1] != "claude") {
		return errors.New("usage: login NAME codex|claude")
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	p, err := store.profile(args[0])
	if err != nil {
		return err
	}
	root, command := p.CodexHome, "codex"
	loginArgs := []string{"login"}
	if args[1] == "claude" {
		root, command = p.ClaudeConfigDir, "claude"
		loginArgs = []string{"auth", "login"}
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	cmd := exec.Command(command, loginArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), envName(args[1])+"="+root)
	return cmd.Run()
}

func envName(provider string) string {
	if provider == "codex" {
		return "CODEX_HOME"
	}
	return "CLAUDE_CONFIG_DIR"
}

func snapshotCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: snapshot NAME [--json]")
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	p, err := store.profile(args[0])
	if err != nil {
		return err
	}
	all := []ProviderSnapshot{probe(p.Name, "codex", p.CodexHome), probe(p.Name, "claude", p.ClaudeConfigDir)}
	if len(args) > 1 && args[1] == "--json" {
		return printJSON(all)
	}
	for _, s := range all {
		if s.Error != "" {
			fmt.Printf("%s: unavailable (%s)\n", s.Provider, s.Error)
		} else {
			fmt.Printf("%s: %s %s; auth=%s; usage=%s\n", s.Provider, s.Status, s.Version, s.Authenticated, s.Usage)
		}
	}
	return nil
}

func probe(profile, provider, root string) ProviderSnapshot {
	s := ProviderSnapshot{Provider: provider, Profile: profile, Checked: time.Now().UTC(), Status: "unavailable", Authenticated: "unavailable", Usage: "unavailable: provider quota API not exposed by CLI"}
	command := provider
	cmd := exec.Command(command, "--version")
	cmd.Env = append(os.Environ(), envName(provider)+"="+root)
	out, err := cmd.Output()
	if err != nil {
		s.Error = fmt.Sprintf("%s executable unavailable", provider)
		return s
	}
	s.Status = "available"
	s.Version = strings.TrimSpace(string(out))
	statusArgs := []string{"login", "status"}
	if provider == "claude" {
		statusArgs = []string{"auth", "status"}
	}
	statusCmd := exec.Command(command, statusArgs...)
	statusCmd.Env = append(os.Environ(), envName(provider)+"="+root)
	if _, err := statusCmd.Output(); err == nil {
		s.Authenticated = "authenticated"
	} else {
		s.Authenticated = "not authenticated"
	}
	return s
}

func refreshCommand(args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("usage: refresh NAME|--all")
	}
	profiles := store.registry.Profiles
	if args[0] != "--all" {
		p, err := store.profile(args[0])
		if err != nil {
			return err
		}
		profiles = []Profile{p}
	}
	for _, p := range profiles {
		path := filepath.Join(store.eventsDir, p.Name+"-latest.json")
		snapshots := []ProviderSnapshot{probe(p.Name, "codex", p.CodexHome), probe(p.Name, "claude", p.ClaudeConfigDir)}
		if err := atomicJSON(path, snapshots, 0o600); err != nil {
			return err
		}
		for i := range store.registry.Profiles {
			if store.registry.Profiles[i].Name == p.Name {
				store.registry.Profiles[i].LastRefresh = time.Now().UTC()
			}
		}
	}
	if err := store.save(); err != nil {
		return err
	}
	return nil
}

func dashboardCommand(args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	if len(store.registry.Profiles) == 0 {
		fmt.Println("No profiles. Create one with: agent-profile profile create NAME")
		return nil
	}
	rows := make([]tui.Row, 0, len(store.registry.Profiles))
	for _, p := range store.registry.Profiles {
		snapshots := []ProviderSnapshot{probe(p.Name, "codex", p.CodexHome), probe(p.Name, "claude", p.ClaudeConfigDir)}
		rows = append(rows, tui.Row{Profile: p.Name, Codex: snapshotLabel(snapshots[0]), Claude: snapshotLabel(snapshots[1])})
	}
	if len(args) > 0 && args[0] == "--watch" {
		return tui.Run(rows)
	}
	for _, row := range rows {
		fmt.Printf("%-22s %-21s %s\n", row.Profile, row.Codex, row.Claude)
	}
	return nil
}

func snapshotLabel(s ProviderSnapshot) string {
	if s.Error != "" {
		return "unavailable"
	}
	return s.Status
}

func serviceCommand(args []string) error {
	if len(args) < 1 || args[0] != "install" {
		return errors.New("usage: service install [--apply]")
	}
	fs := flag.NewFlagSet("service install", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "write service files")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	root, err := dataRoot()
	if err != nil {
		return err
	}
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		dir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path := filepath.Join(dir, "Library", "LaunchAgents", "com.isnakolah.agent-profile.plist")
		content := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?><plist version=\"1.0\"><dict><key>Label</key><string>com.isnakolah.agent-profile</string><key>ProgramArguments</key><array><string>%s</string><string>refresh</string><string>--all</string></array><key>RunAtLoad</key><true/><key>StartInterval</key><integer>300</integer></dict></plist>\n", executable)
		return writeService(path, content, *apply)
	}
	path := filepath.Join(root, "systemd", "user", "agent-profile.service")
	content := fmt.Sprintf("[Unit]\nDescription=Agent Profile refresh\n\n[Service]\nType=oneshot\nExecStart=%s refresh --all\n", executable)
	return writeService(path, content, *apply)
}

func writeService(path, content string, apply bool) error {
	if !apply {
		fmt.Printf("%s\n%s", path, content)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

func notifyCommand(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: notify NAME MESSAGE")
	}
	message := strings.Join(args[1:], " ")
	if runtime.GOOS == "darwin" {
		return exec.Command("osascript", "-e", fmt.Sprintf("display notification %q with title %q", message, "agent-profile "+args[0])).Run()
	}
	if _, err := exec.LookPath("notify-send"); err != nil {
		return fmt.Errorf("native notification unavailable: %w", err)
	}
	return exec.Command("notify-send", "agent-profile "+args[0], message).Run()
}

func doctorCommand(args []string) error {
	store, err := openStore()
	if err != nil {
		return err
	}
	type report struct {
		Version  string `json:"version"`
		OS       string `json:"os"`
		Root     string `json:"data_root"`
		Profiles int    `json:"profiles"`
		Codex    string `json:"codex"`
		Claude   string `json:"claude"`
	}
	r := report{Version: version, OS: runtime.GOOS, Root: store.root, Profiles: len(store.registry.Profiles), Codex: executableStatus("codex"), Claude: executableStatus("claude")}
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

type store struct {
	root         string
	registryPath string
	eventsDir    string
	registry     Registry
}

func openStore() (*store, error) {
	root, err := dataRoot()
	if err != nil {
		return nil, err
	}
	s := &store{root: root, registryPath: filepath.Join(root, "profiles.json"), eventsDir: filepath.Join(root, "snapshots")}
	if err := os.MkdirAll(s.eventsDir, 0o700); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.registryPath)
	if errors.Is(err, os.ErrNotExist) {
		s.registry = Registry{SchemaVersion: 1, Profiles: []Profile{}}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.registry); err != nil {
		return nil, fmt.Errorf("read profile registry: %w", err)
	}
	return s, nil
}

func (s *store) create(name string) error {
	if !validName(name) {
		return errors.New("profile name must contain only letters, digits, dot, dash, or underscore")
	}
	if _, err := s.profile(name); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	}
	base := filepath.Join(s.root, "profiles", name)
	p := Profile{Name: name, CreatedAt: time.Now().UTC(), CodexHome: filepath.Join(base, "codex"), ClaudeConfigDir: filepath.Join(base, "claude")}
	for _, dir := range []string{p.CodexHome, p.ClaudeConfigDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	s.registry.Profiles = append(s.registry.Profiles, p)
	sort.Slice(s.registry.Profiles, func(i, j int) bool { return s.registry.Profiles[i].Name < s.registry.Profiles[j].Name })
	if err := s.save(); err != nil {
		return err
	}
	fmt.Println("created", name)
	return nil
}

func (s *store) remove(name string) error {
	for i, p := range s.registry.Profiles {
		if p.Name == name {
			if err := os.RemoveAll(filepath.Join(s.root, "profiles", name)); err != nil {
				return err
			}
			s.registry.Profiles = append(s.registry.Profiles[:i], s.registry.Profiles[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("profile %q not found", name)
}

func (s *store) profile(name string) (Profile, error) {
	for _, p := range s.registry.Profiles {
		if p.Name == name {
			return p, nil
		}
	}
	return Profile{}, fmt.Errorf("profile %q not found", name)
}

func (s *store) save() error { return atomicJSON(s.registryPath, s.registry, 0o600) }

func atomicJSON(path string, value any, mode os.FileMode) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp-" + fmt.Sprint(os.Getpid())
	if err := os.WriteFile(tmp, append(b, '\n'), mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func dataRoot() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "agent-profile"), nil
}

func launcherDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func validName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if !(r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func printJSON(value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
