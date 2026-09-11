package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/credentials"
	"github.com/isnakolah/agent-profile/internal/session"
	"github.com/isnakolah/agent-profile/internal/state"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type store struct {
	root         string
	registryPath string
	eventsDir    string
	registry     Registry
	lock         *os.File
}

func openStore() (*store, error) {
	root, err := dataRoot()
	if err != nil {
		return nil, err
	}
	lock, err := state.Lock(filepath.Join(root, "registry.lock"))
	if err != nil {
		return nil, err
	}
	s := &store{lock: lock, root: root, registryPath: filepath.Join(root, "profiles.json"), eventsDir: filepath.Join(root, "snapshots")}
	if err := state.PrivateDir(s.eventsDir); err != nil {
		s.close()
		return nil, err
	}
	if info, e := os.Lstat(s.registryPath); e == nil && info.Mode()&os.ModeSymlink != 0 {
		s.close()
		return nil, errors.New("registry must not be a symlink")
	}
	b, err := os.ReadFile(s.registryPath)
	if errors.Is(err, os.ErrNotExist) {
		s.registry = Registry{SchemaVersion: 2, Profiles: []Profile{}}
		return s, nil
	}
	if err != nil {
		s.close()
		return nil, err
	}
	if err := json.Unmarshal(b, &s.registry); err != nil {
		s.close()
		return nil, fmt.Errorf("read profile registry: %w", err)
	}
	if s.registry.SchemaVersion != 1 && s.registry.SchemaVersion != 2 {
		s.close()
		return nil, errors.New("unsupported registry schema")
	}
	seen := map[string]bool{}
	for _, p := range s.registry.Profiles {
		if seen[p.Name] {
			s.close()
			return nil, errors.New("duplicate profile in registry")
		}
		seen[p.Name] = true
		for kind, mode := range p.Auth {
			if (kind != "codex" && kind != "claude") || (mode != "login" && mode != "api-key") {
				s.close()
				return nil, errors.New("unsupported profile authentication mode")
			}
		}
		if err := safeProfilePath(root, p.Name); err != nil {
			s.close()
			return nil, err
		}
		if !validName(p.Name) || p.CodexHome != filepath.Join(root, "profiles", p.Name, "codex") || p.ClaudeConfigDir != filepath.Join(root, "profiles", p.Name, "claude") {
			s.close()
			return nil, errors.New("unsafe profile registry paths")
		}
	}
	if s.registry.SchemaVersion == 1 {
		if err := state.Write(s.registryPath+".v1.bak", b, 0600); err != nil {
			s.close()
			return nil, err
		}
		s.registry.SchemaVersion = 2
		if err := s.save(); err != nil {
			s.close()
			return nil, err
		}
	}
	return s, nil
}
func (s *store) close() { state.Unlock(s.lock); s.lock = nil }

func (s *store) create(name string) error {
	if !validName(name) {
		return errors.New("profile name must contain only letters, digits, dot, dash, or underscore")
	}
	if _, err := s.profile(name); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	}
	if err := safeProfilePath(s.root, name); err != nil {
		return err
	}
	base := filepath.Join(s.root, "profiles", name)
	p := Profile{Name: name, CreatedAt: time.Now().UTC(), CodexHome: filepath.Join(base, "codex"), ClaudeConfigDir: filepath.Join(base, "claude")}
	for _, dir := range []string{filepath.Join(s.root, "profiles"), base, p.CodexHome, p.ClaudeConfigDir} {
		if err := state.PrivateDir(dir); err != nil {
			return err
		}
	}
	s.registry.Profiles = append(s.registry.Profiles, p)
	sort.Slice(s.registry.Profiles, func(i, j int) bool { return s.registry.Profiles[i].Name < s.registry.Profiles[j].Name })
	if err := s.save(); err != nil {
		return err
	}
	if err := appendEvent(s, Event{At: time.Now().UTC(), Action: "profile.create", Profile: name}); err != nil {
		return err
	}
	fmt.Println("created", name)
	return nil
}

func (s *store) remove(name string) error {
	if !validName(name) {
		return errors.New("invalid profile name")
	}
	if err := safeProfilePath(s.root, name); err != nil {
		return err
	}
	all, err := session.List(s.root)
	if err != nil {
		return err
	}
	for _, v := range all {
		if v.Profile == name && v.State == "running" {
			return errors.New("stop all profile sessions before removing the profile")
		}
	}
	for i, p := range s.registry.Profiles {
		if p.Name == name {
			for kind, mode := range p.Auth {
				if mode == "api-key" {
					if err := credentials.Delete(s.root, name, kind); err != nil {
						return err
					}
				}
			}
			if err := os.RemoveAll(filepath.Join(s.root, "profiles", name)); err != nil {
				return err
			}
			s.registry.Profiles = append(s.registry.Profiles[:i], s.registry.Profiles[i+1:]...)
			if err := s.save(); err != nil {
				return err
			}
			files, _ := launcherFiles(name)
			for path := range files {
				if b, e := os.ReadFile(path); e == nil && (strings.Contains(string(b), launcherMarker) || legacyLauncher(path, name, string(b))) {
					if e = os.Remove(path); e != nil {
						return e
					}
				}
			}
			return appendEvent(s, Event{At: time.Now().UTC(), Action: "profile.remove", Profile: name})
		}
	}
	return fmt.Errorf("profile %q not found", name)
}

func (s *store) setReview(name string, when time.Time) error {
	for i := range s.registry.Profiles {
		if s.registry.Profiles[i].Name == name {
			s.registry.Profiles[i].CredentialReview = when.UTC().Format(time.RFC3339)
			if err := s.save(); err != nil {
				return err
			}
			return appendEvent(s, Event{At: time.Now().UTC(), Action: "profile.review", Profile: name, Detail: when.UTC().Format(time.RFC3339)})
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

func appendEvent(s *store, event Event) error {
	path := filepath.Join(s.root, "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_APPEND|unix.O_WRONLY|unix.O_NOFOLLOW, 0600)
	var f *os.File
	if err == nil {
		f = os.NewFile(uintptr(fd), path)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func atomicJSON(path string, value any, mode os.FileMode) error { return state.JSON(path, value) }
func dataRoot() (string, error)                                 { return state.Root() }

func launcherDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func validName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, "-") || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if !(r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func safeProfilePath(root, name string) error {
	if !validName(name) {
		return errors.New("invalid profile name")
	}
	for _, path := range []string{filepath.Join(root, "profiles"), filepath.Join(root, "profiles", name), filepath.Join(root, "profiles", name, "codex"), filepath.Join(root, "profiles", name, "claude")} {
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe profile directory: %s", path)
		}
	}
	return nil
}
