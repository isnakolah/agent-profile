package app

import (
	"errors"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/credentials"
	"github.com/isnakolah/agent-profile/internal/provider"
	"golang.org/x/term"
	"io"
	"os"
	"strings"
)

func authCommand(args []string) error {
	if len(args) != 3 || !provider.Valid(args[1]) {
		return errors.New("usage: auth NAME codex|claude login|key|clear")
	}
	name, kind, action := args[0], args[1], args[2]
	s, e := openStore()
	if e != nil {
		return e
	}
	p, e := s.profile(name)
	root := s.root
	s.close()
	if e != nil {
		return e
	}
	switch action {
	case "login":
		if e = loginCommand(args[:2]); e != nil {
			return e
		}
	case "key":
		var b []byte
		if term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprint(os.Stderr, "API key (stored in your OS vault): ")
			b, e = term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
		} else {
			b, e = io.ReadAll(io.LimitReader(os.Stdin, 16385))
		}
		if e != nil {
			return e
		}
		key := strings.TrimSpace(string(b))
		if len(key) == 0 || len(key) > 16384 {
			return errors.New("API key must be nonempty and at most 16 KiB")
		}
		if e = credentials.Set(root, name, kind, key); e != nil {
			return fmt.Errorf("store API key: %w", e)
		}
	case "clear":
		if e = credentials.Delete(root, name, kind); e != nil {
			return e
		}
	default:
		return errors.New("auth action must be login, key or clear")
	}
	s, e = openStore()
	if e != nil {
		return e
	}
	defer s.close()
	for i := range s.registry.Profiles {
		if s.registry.Profiles[i].Name == p.Name {
			if s.registry.Profiles[i].Auth == nil {
				s.registry.Profiles[i].Auth = map[string]string{}
			}
			mode := "login"
			if action == "key" {
				mode = "api-key"
			}
			s.registry.Profiles[i].Auth[kind] = mode
			return s.save()
		}
	}
	return errors.New("profile was removed during authentication")
}
func profileEnvironment(s *store, p Profile, kind string) ([]string, []string, error) {
	key := ""
	var e error
	mode := p.Auth[kind]
	if mode == "api-key" {
		key, e = credentials.Get(s.root, p.Name, kind)
		if e != nil {
			return nil, nil, e
		}
	}
	root := p.CodexHome
	if kind == "claude" {
		root = p.ClaudeConfigDir
	}
	return provider.Environment(os.Environ(), kind, root, key), provider.Arguments(kind, mode, nil), nil
}
