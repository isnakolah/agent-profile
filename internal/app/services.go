package app

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/state"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const serviceLabel = "com.isnakolah.agent-profile"

func xmlText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func serviceCommand(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: service install|enable|disable|status|uninstall [--apply]")
	}
	action := args[0]
	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "apply service changes")
	if e := fs.Parse(args[1:]); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected service arguments")
	}
	if action != "install" {
		return serviceControl(action, *apply)
	}
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	if runtime.GOOS == "darwin" {
		return writeService(filepath.Join(home, "Library", "LaunchAgents", serviceLabel+".plist"), launchdServiceContent(executable), *apply)
	}
	base, e := os.UserConfigDir()
	if e != nil {
		return e
	}
	dir := filepath.Join(base, "systemd", "user")
	if e = writeService(filepath.Join(dir, "agent-profile.service"), systemdServiceContent(executable), *apply); e != nil {
		return e
	}
	return writeService(filepath.Join(dir, "agent-profile.timer"), systemdTimerContent(), *apply)
}
func launchdServiceContent(executable string) string {
	root, _ := dataRoot()
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>%s</string>
<key>ProgramArguments</key><array><string>%s</string><string>refresh</string><string>--all</string></array>
<key>RunAtLoad</key><true/><key>StartInterval</key><integer>300</integer>
<key>EnvironmentVariables</key><dict><key>PATH</key><string>%s</string><key>AGENT_PROFILE_HOME</key><string>%s</string></dict>
</dict></plist>
`, serviceLabel, xmlText(executable), xmlText(os.Getenv("PATH")), xmlText(root))
}
func unitQuote(s string) string {
	return strconv.Quote(strings.ReplaceAll(strings.ReplaceAll(s, "%", "%%"), "$", "$$"))
}
func systemdServiceContent(executable string) string {
	root, _ := dataRoot()
	return fmt.Sprintf("[Unit]\nDescription=Agent Profile status refresh\n\n[Service]\nType=oneshot\nExecStart=%s refresh --all\nEnvironment=%s\nEnvironment=%s\n", unitQuote(executable), unitQuote("AGENT_PROFILE_HOME="+root), unitQuote("PATH="+os.Getenv("PATH")))
}
func systemdTimerContent() string {
	return "[Unit]\nDescription=Refresh Agent Profile snapshots\n\n[Timer]\nOnBootSec=2min\nOnUnitActiveSec=5min\nPersistent=true\n\n[Install]\nWantedBy=timers.target\n"
}
func writeService(path, content string, apply bool) error {
	if !apply {
		fmt.Printf("%s\n%s", path, content)
		return nil
	}
	if e := state.Write(path, []byte(content), 0600); e != nil {
		return e
	}
	fmt.Println(path)
	return nil
}
func serviceControl(action string, apply bool) error {
	if action != "status" && !apply {
		return errors.New("service changes require --apply")
	}
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	var commands [][]string
	if runtime.GOOS == "darwin" {
		domain := fmt.Sprintf("gui/%d", os.Getuid())
		path := filepath.Join(home, "Library", "LaunchAgents", serviceLabel+".plist")
		switch action {
		case "enable":
			commands = [][]string{{"launchctl", "bootstrap", domain, path}}
		case "disable":
			commands = [][]string{{"launchctl", "bootout", domain + "/" + serviceLabel}}
		case "status":
			commands = [][]string{{"launchctl", "print", domain + "/" + serviceLabel}}
		case "uninstall":
			_ = exec.Command("launchctl", "bootout", domain+"/"+serviceLabel).Run()
			return os.Remove(path)
		default:
			return errors.New("unknown service action")
		}
	} else {
		switch action {
		case "enable":
			commands = [][]string{{"systemctl", "--user", "daemon-reload"}, {"systemctl", "--user", "enable", "--now", "agent-profile.timer"}}
		case "disable":
			commands = [][]string{{"systemctl", "--user", "disable", "--now", "agent-profile.timer"}}
		case "status":
			commands = [][]string{{"systemctl", "--user", "status", "agent-profile.timer"}}
		case "uninstall":
			_ = exec.Command("systemctl", "--user", "disable", "--now", "agent-profile.timer").Run()
			base, e := os.UserConfigDir()
			if e != nil {
				return e
			}
			for _, name := range []string{"agent-profile.service", "agent-profile.timer"} {
				if e = os.Remove(filepath.Join(base, "systemd", "user", name)); e != nil && !os.IsNotExist(e) {
					return e
				}
			}
			return exec.Command("systemctl", "--user", "daemon-reload").Run()
		default:
			return errors.New("unknown service action")
		}
	}
	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if e = cmd.Run(); e != nil {
			return e
		}
	}
	return nil
}
