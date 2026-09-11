package app

import (
	"errors"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/provider"
	"os"
	"os/exec"
	"time"
)

func profileCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: profile create|list|show|remove NAME")
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.close()
	switch args[0] {
	case "create":
		if len(args) != 2 {
			return errors.New("usage: profile create NAME")
		}
		if err := checkLaunchers(args[1]); err != nil {
			return err
		}
		if err := store.create(args[1]); err != nil {
			return err
		}
		store.close()
		return launcherCommand([]string{"install", args[1], "--apply"})
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
	case "review":
		if len(args) != 3 {
			return errors.New("usage: profile review NAME RFC3339")
		}
		when, err := time.Parse(time.RFC3339, args[2])
		if err != nil {
			return fmt.Errorf("review date must be RFC3339: %w", err)
		}
		return store.setReview(args[1], when)
	default:
		return fmt.Errorf("unknown profile command %q", args[0])
	}
}

func loginCommand(args []string) error {
	if len(args) != 2 || (args[1] != "codex" && args[1] != "claude") {
		return errors.New("usage: login NAME codex|claude")
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.close()
	p, err := store.profile(args[0])
	if err != nil {
		return err
	}
	command, loginArgs, err := providerLoginSpec(args[1])
	if err != nil {
		return err
	}
	root := p.CodexHome
	if args[1] == "claude" {
		root = p.ClaudeConfigDir
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	cmd := exec.Command(command, loginArgs...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = provider.Environment(os.Environ(), args[1], root, "")
	store.close()
	if err := cmd.Run(); err != nil {
		return err
	}
	return appendEvent(store, Event{At: time.Now().UTC(), Action: "login", Profile: p.Name, Detail: args[1]})
}

func providerLoginSpec(provider string) (string, []string, error) {
	switch provider {
	case "codex":
		return "codex", []string{"login"}, nil
	case "claude":
		return "claude", []string{"auth", "login"}, nil
	default:
		return "", nil, fmt.Errorf("unsupported provider %q", provider)
	}
}

func envName(provider string) string {
	if provider == "codex" {
		return "CODEX_HOME"
	}
	return "CLAUDE_CONFIG_DIR"
}
