package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/provider"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func snapshotCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: snapshot NAME [--json]")
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
	all := []ProviderSnapshot{probe(p.Name, "codex", p.CodexHome), probe(p.Name, "claude", p.ClaudeConfigDir)}
	for i := range all {
		if p.Auth[all[i].Provider] == "api-key" {
			all[i].Authenticated = "API key configured (not verified)"
		}
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command, "--version")
	cmd.Env = isolatedEnvironment(provider, root)
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
	statusCmd := exec.CommandContext(ctx, command, statusArgs...)
	statusCmd.Env = isolatedEnvironment(provider, root)
	if _, err := statusCmd.Output(); err == nil {
		s.Authenticated = "authenticated"
	} else {
		s.Authenticated = "unknown (login status failed)"
	}
	return s
}

func refreshCommand(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: refresh NAME|--all")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	profiles := append([]Profile(nil), s.registry.Profiles...)
	root := s.root
	if args[0] != "--all" {
		p, e := s.profile(args[0])
		if e != nil {
			s.close()
			return e
		}
		profiles = []Profile{p}
	}
	s.close()
	for _, p := range profiles {
		snapshots := []ProviderSnapshot{probe(p.Name, "codex", p.CodexHome), probe(p.Name, "claude", p.ClaudeConfigDir)}
		for i := range snapshots {
			if p.Auth[snapshots[i].Provider] == "api-key" {
				snapshots[i].Authenticated = "API key configured (not verified)"
			}
		}
		current, e := openStore()
		if e != nil {
			return e
		}
		if _, e = current.profile(p.Name); e != nil {
			current.close()
			continue
		}
		if e = atomicJSON(filepath.Join(root, "snapshots", p.Name+"-latest.json"), snapshots, 0600); e != nil {
			current.close()
			return e
		}
		for i := range current.registry.Profiles {
			if current.registry.Profiles[i].Name == p.Name {
				current.registry.Profiles[i].LastRefresh = time.Now().UTC()
			}
		}
		e = current.save()
		if e == nil {
			e = appendEvent(current, Event{At: time.Now().UTC(), Action: "refresh", Profile: p.Name})
		}
		current.close()
		if e != nil {
			return e
		}
	}
	return nil
}

func daemonCommand(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	interval := fs.Duration("interval", 5*time.Minute, "refresh interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *interval <= 0 {
		return errors.New("daemon interval must be positive")
	}
	if err := refreshCommand([]string{"--all"}); err != nil {
		return err
	}
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := refreshCommand([]string{"--all"}); err != nil {
			fmt.Fprintln(os.Stderr, "refresh:", err)
		}
	}
	return nil
}

func isolatedEnvironment(name, root string) []string {
	return provider.Environment(os.Environ(), name, root, "")
}
