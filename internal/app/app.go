package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/session"
	"io"
	"os"
	"os/exec"
)

func Main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "agent-profile:", err)
		var exit session.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.Code)
		}
		var processExit *exec.ExitError
		if errors.As(err, &processExit) {
			os.Exit(processExit.ExitCode())
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return dashboardCommand([]string{"--watch"})
	}
	if args[0] == "help" || args[0] == "--help" {
		usage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "_session-worker":
		if len(args) != 3 {
			return errors.New("invalid worker arguments")
		}
		return session.Serve(args[1], args[2])
	case "run":
		return runProfile(args[1:])
	case "session":
		return sessionCommand(args[1:])
	case "auth":
		return authCommand(args[1:])
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
	case "daemon":
		return daemonCommand(args[1:])
	case "login":
		return loginCommand(args[1:])
	case "dashboard":
		return dashboardCommand(args[1:])
	case "service":
		return serviceCommand(args[1:])
	case "notify":
		return notifyCommand(args[1:])
	default:
		return runProfile(args)
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `agent-profile — isolated Codex and Claude profile manager

Commands:
  run NAME codex|claude [PROVIDER ARGS...]
  session list [--json] | attach ID [--view] | stop ID | remove ID
  auth NAME codex|claude login|key|clear
  profile create|list|show|remove|review NAME [RFC3339]
  launcher install NAME [--apply]
  login NAME codex|claude
  snapshot NAME [--json]
  refresh [NAME|--all]
  daemon [--interval DURATION]
  dashboard [--watch]
  doctor [--json]
  service install [--apply]
  notify NAME MESSAGE

Profile state contains metadata only. Provider auth stays in isolated runtime roots.`)
}

func printJSON(value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
