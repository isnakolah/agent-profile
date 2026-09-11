package app

import (
	"errors"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/provider"
	"github.com/isnakolah/agent-profile/internal/session"
	"github.com/isnakolah/agent-profile/internal/tui"
	"golang.org/x/term"
	"os"
	"os/exec"
	"path/filepath"
)

func runProfile(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: agent-profile run NAME codex|claude [ARGS...]")
	}
	if len(args) == 1 {
		return profileDashboard(args[0])
	}
	name, kind := args[0], args[1]
	if !provider.Valid(kind) {
		return errors.New("provider must be codex or claude")
	}
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
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	cwd, e = filepath.EvalSymlinks(cwd)
	if e != nil {
		return e
	}
	if len(args) == 2 && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
		all, e := session.List(root)
		if e != nil {
			return e
		}
		items := []tui.Item{{ID: "new", Title: "New session", Detail: "Start " + kind + " in " + cwd}}
		for _, v := range all {
			if v.Profile == name && v.Provider == kind && v.Directory == cwd {
				items = append(items, sessionItem(v))
			}
		}
		if len(items) > 1 {
			choice, e := tui.Choose(name+" / "+kind, "Choose a session · "+cwd, items)
			if e != nil {
				return e
			}
			if choice.ID == "" {
				return nil
			}
			if choice.ID != "new" {
				return openSession(root, choice.ID)
			}
		}
	}
	return launch(s, p, kind, cwd, args[2:])
}
func launch(s *store, p Profile, kind, cwd string, args []string) error {
	env, prefix, e := profileEnvironment(s, p, kind)
	if e != nil {
		return e
	}
	command, e := exec.LookPath(kind)
	if e != nil {
		return fmt.Errorf("install %s before starting a session: %w", kind, e)
	}
	argv := append(prefix, args...)
	if provider.Direct(kind, args) || !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		cmd := exec.Command(command, argv...)
		cmd.Env = env
		cmd.Dir = cwd
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		return cmd.Run()
	}
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	w, h, _ := term.GetSize(int(os.Stdin.Fd()))
	info, e := session.Start(s.root, executable, session.Launch{Profile: p.Name, Provider: kind, Directory: cwd, Executable: command, Args: argv, Env: env, Width: w, Height: h})
	if e != nil {
		return e
	}
	return session.Attach(s.root, info.ID, false)
}
func sessionItem(v session.Info) tui.Item {
	detail := v.Provider + " · " + v.Directory
	status := v.State
	if v.Controller != "" {
		status = "attached · " + v.Controller
	} else if v.State == "running" {
		status = "detached"
	}
	if v.Viewers > 0 {
		status += fmt.Sprintf(" · %d viewers", v.Viewers)
	}
	return tui.Item{ID: v.ID, Title: v.Profile + " / " + v.Provider, Detail: detail, Status: status, Hint: v.Created.Local().Format("Jan 2 · 15:04")}
}
func openSession(root, id string) error {
	v, e := session.Find(root, id)
	if e != nil {
		return e
	}
	if v.State == "running" {
		view := false
		if v.Controller != "" {
			item, e := tui.Choose("Session already attached", v.Controller, []tui.Item{{ID: "view", Title: "View only", Detail: "Watch without sending keyboard input"}, {ID: "control", Title: "Take control", Detail: "Transfer keyboard control; other terminals become viewers"}})
			if e != nil || item.ID == "" {
				return e
			}
			view = item.ID == "view"
			if !view {
				return attachWithControl(root, id)
			}
		}
		return session.Attach(root, id, view)
	}
	choice, e := tui.Choose("Recover session", v.Profile+" / "+v.Provider, []tui.Item{{ID: "resume", Title: "Open provider resume picker", Detail: "Select the conversation to recover. The original prompt is never replayed."}, {ID: "new", Title: "Start a new conversation", Detail: v.Directory}})
	if e != nil || choice.ID == "" {
		return e
	}
	s, e := openStore()
	if e != nil {
		return e
	}
	p, e := s.profile(v.Profile)
	s.close()
	if e != nil {
		return e
	}
	args := []string{}
	if choice.ID == "resume" {
		args = provider.Resume(v.Provider)
	}
	return launch(s, p, v.Provider, v.Directory, args)
}
func attachWithControl(root, id string) error { return session.AttachControl(root, id) }
func sessionCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: session list [--json] | attach ID [--view] | stop ID | remove ID")
	}
	root, e := dataRoot()
	if e != nil {
		return e
	}
	if args[0] == "list" {
		all, e := session.List(root)
		if e != nil {
			return e
		}
		if len(args) == 2 && args[1] == "--json" {
			return printJSON(all)
		}
		if len(args) > 1 {
			return errors.New("usage: session list [--json]")
		}
		for _, v := range all {
			fmt.Printf("%s  %-12s %-7s %-12s %s\n", v.ID, v.Profile, v.Provider, sessionItem(v).Status, v.Directory)
		}
		return nil
	}
	if len(args) < 2 {
		return errors.New("session ID required")
	}
	switch args[0] {
	case "attach":
		if len(args) == 3 && args[2] == "--view" {
			return session.Attach(root, args[1], true)
		}
		if len(args) != 2 {
			return errors.New("usage: session attach ID [--view]")
		}
		return openSession(root, args[1])
	case "stop":
		return session.Stop(root, args[1])
	case "remove":
		return session.Remove(root, args[1])
	}
	return errors.New("unknown session action")
}
