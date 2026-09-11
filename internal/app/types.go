package app

import (
	"runtime/debug"
	"time"
)

var version = buildVersion()

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && len(setting.Value) >= 12 {
				return "0.1.0-dev+" + setting.Value[:12]
			}
		}
	}
	return "0.1.0-dev"
}

type Profile struct {
	Name             string            `json:"name"`
	CreatedAt        time.Time         `json:"created_at"`
	LastRefresh      time.Time         `json:"last_refresh,omitempty"`
	CredentialReview string            `json:"credential_review,omitempty"`
	CodexHome        string            `json:"codex_home"`
	ClaudeConfigDir  string            `json:"claude_config_dir"`
	Auth             map[string]string `json:"auth,omitempty"`
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

type Event struct {
	At      time.Time `json:"at"`
	Action  string    `json:"action"`
	Profile string    `json:"profile,omitempty"`
	Detail  string    `json:"detail,omitempty"`
}

type DoctorReport struct {
	Version  string `json:"version"`
	OS       string `json:"os"`
	Root     string `json:"data_root"`
	Profiles int    `json:"profiles"`
	Codex    string `json:"codex"`
	Claude   string `json:"claude"`
}
