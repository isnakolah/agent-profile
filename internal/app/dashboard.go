package app

import (
	"encoding/json"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/session"
	"github.com/isnakolah/agent-profile/internal/tui"
	"golang.org/x/term"
	"os"
	"path/filepath"
	"time"
)

func dashboardCommand(args []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return profileCommand([]string{"list"})
	}
	for {
		choice, e := tui.ChooseLive("Agent Profile", "Your accounts. Your sessions. One place.", nil, func() ([]tui.Item, error) {
			s, e := openStore()
			if e != nil {
				return nil, e
			}
			profiles := append([]Profile(nil), s.registry.Profiles...)
			root := s.root
			s.close()
			all, e := session.List(root)
			if e != nil {
				return nil, e
			}
			items := []tui.Item{}
			for _, p := range profiles {
				running := 0
				for _, v := range all {
					if v.Profile == p.Name && v.State == "running" {
						running++
					}
				}
				status := fmt.Sprintf("%d active sessions", running)
				items = append(items, tui.Item{ID: p.Name, Title: p.Name, Detail: cachedLabel(root, p), Status: status})
			}
			items = append(items, tui.Item{ID: "+", Title: "Create profile", Detail: "Separate credentials for work, personal, or another account"})
			return items, nil
		})
		if e != nil || choice.ID == "" {
			return e
		}
		if choice.ID == "+" {
			name, e := tui.Input("Name your profile")
			if e != nil {
				return e
			}
			if name != "" {
				if e = profileCommand([]string{"create", name}); e != nil {
					showError(e)
				}
			}
		} else {
			if e = profileDashboard(choice.ID); e != nil {
				showError(e)
			}
		}
	}
}
func cachedLabel(root string, p Profile) string {
	b, e := os.ReadFile(filepath.Join(root, "snapshots", p.Name+"-latest.json"))
	if e != nil {
		return "Codex + Claude · not checked yet"
	}
	var snapshots []ProviderSnapshot
	if json.Unmarshal(b, &snapshots) != nil {
		return "Status cache needs refresh"
	}
	label := ""
	for _, v := range snapshots {
		if label != "" {
			label += " · "
		}
		label += v.Provider + ": " + v.Authenticated
		if time.Since(v.Checked) > 10*time.Minute {
			label += " (stale)"
		}
	}
	return label
}
func profileDashboard(name string) error {
	root, e := dataRoot()
	if e != nil {
		return e
	}
	for {
		choice, e := tui.ChooseLive(name, "Native provider sessions · credentials stay isolated", nil, func() ([]tui.Item, error) {
			s, e := openStore()
			if e != nil {
				return nil, e
			}
			_, e = s.profile(name)
			s.close()
			if e != nil {
				return nil, e
			}
			items := []tui.Item{{ID: "codex", Title: "Open Codex", Detail: "New session or pick up where you left off"}, {ID: "claude", Title: "Open Claude", Detail: "New session or pick up where you left off"}}
			all, e := session.List(root)
			if e != nil {
				return nil, e
			}
			for _, v := range all {
				if v.Profile == name {
					items = append(items, sessionItem(v))
				}
			}
			items = append(items, tui.Item{ID: "settings", Title: "Profile settings", Detail: "Authentication, launchers, status and session management"})
			return items, nil
		})
		if e != nil || choice.ID == "" {
			return e
		}
		switch choice.ID {
		case "codex", "claude":
			e = runProfile([]string{name, choice.ID})
		case "settings":
			e = profileSettings(name)
		default:
			e = openSession(root, choice.ID)
		}
		if e != nil {
			showError(e)
		}
	}
}
func showError(e error) {
	_, _ = tui.Choose("Needs attention", e.Error(), []tui.Item{{ID: "back", Title: "Back", Detail: "Your existing sessions are preserved"}})
}
func profileSettings(name string) error {
	choice, e := tui.Choose(name+" / settings", "Changes apply to new provider sessions", []tui.Item{{ID: "codex-login", Title: "Sign in to Codex"}, {ID: "claude-login", Title: "Sign in to Claude"}, {ID: "codex-key", Title: "Set Codex API key", Detail: "Store in OS credential vault"}, {ID: "claude-key", Title: "Set Claude API key", Detail: "Store in OS credential vault"}, {ID: "refresh", Title: "Refresh provider status"}, {ID: "launcher", Title: "Install profile commands"}, {ID: "sessions", Title: "Manage sessions", Detail: "Stop processes or remove old session entries"}})
	if e != nil || choice.ID == "" {
		return e
	}
	switch choice.ID {
	case "codex-login":
		return authCommand([]string{name, "codex", "login"})
	case "claude-login":
		return authCommand([]string{name, "claude", "login"})
	case "codex-key":
		return authCommand([]string{name, "codex", "key"})
	case "claude-key":
		return authCommand([]string{name, "claude", "key"})
	case "refresh":
		return refreshCommand([]string{name})
	case "launcher":
		return launcherCommand([]string{"install", name, "--apply"})
	case "sessions":
		root, e := dataRoot()
		if e != nil {
			return e
		}
		all, e := session.List(root)
		if e != nil {
			return e
		}
		items := []tui.Item{}
		for _, v := range all {
			if v.Profile == name {
				items = append(items, sessionItem(v))
			}
		}
		chosen, e := tui.Choose("Manage sessions", "Choose a session", items)
		if e != nil || chosen.ID == "" {
			return e
		}
		v, e := session.Find(root, chosen.ID)
		if e != nil {
			return e
		}
		action := "remove"
		if v.State == "running" {
			action = "stop"
		}
		confirm, e := tui.Choose("Confirm "+action, v.Profile+" / "+v.Provider+" · "+v.Directory, []tui.Item{{ID: "back", Title: "Cancel"}, {ID: action, Title: action + " session", Detail: "Provider conversation history is preserved"}})
		if e != nil || confirm.ID != action {
			return e
		}
		return sessionCommand([]string{action, v.ID})
	}
	return nil
}
