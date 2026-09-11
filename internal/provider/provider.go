// Package provider builds isolated provider invocations without changing caller arguments.
package provider

import (
	"fmt"
	"strings"
)

func Valid(name string) bool { return name == "codex" || name == "claude" }
func RootVariable(name string) string {
	if name == "codex" {
		return "CODEX_HOME"
	}
	return "CLAUDE_CONFIG_DIR"
}

// Environment replaces inherited authentication for both providers. Project,
// shell and terminal settings are retained; provider credentials are explicit.
func Environment(base []string, name, root, key string) []string {
	out := []string{}
	blocked := map[string]bool{"CODEX_HOME": true, "CLAUDE_CONFIG_DIR": true, "OPENAI_API_KEY": true, "CODEX_API_KEY": true, "CODEX_ACCESS_TOKEN": true, "OPENAI_BASE_URL": true, "ANTHROPIC_API_KEY": true, "ANTHROPIC_AUTH_TOKEN": true, "ANTHROPIC_BASE_URL": true, "CLAUDE_CODE_OAUTH_TOKEN": true, "CLAUDE_CODE_USE_BEDROCK": true, "CLAUDE_CODE_USE_VERTEX": true, "CLAUDE_CODE_USE_FOUNDRY": true, "ANTHROPIC_PROFILE": true}
	for _, v := range base {
		k, _, _ := strings.Cut(v, "=")
		if !blocked[k] {
			out = append(out, v)
		}
	}
	out = append(out, RootVariable(name)+"="+root)
	if key != "" {
		variable := "OPENAI_API_KEY"
		if name == "claude" {
			variable = "ANTHROPIC_API_KEY"
		}
		out = append(out, variable+"="+key)
	}
	return out
}
func Arguments(name, mode string, args []string) []string {
	if name != "codex" || mode != "api-key" {
		return append([]string(nil), args...)
	}
	// A named provider consumes the key from the environment, bypassing OAuth
	// caches without writing the key to Codex's auth.json.
	out := []string{"-c", `model_provider="agent_profile"`, "-c", `model_providers.agent_profile.name="OpenAI"`, "-c", `model_providers.agent_profile.base_url="https://api.openai.com/v1"`, "-c", `model_providers.agent_profile.env_key="OPENAI_API_KEY"`, "-c", `model_providers.agent_profile.wire_api="responses"`}
	return append(out, args...)
}
func Login(name string) ([]string, error) {
	switch name {
	case "codex":
		return []string{"login"}, nil
	case "claude":
		return []string{"auth", "login"}, nil
	}
	return nil, fmt.Errorf("unsupported provider %q", name)
}
func Resume(name string) []string {
	if name == "codex" {
		return []string{"resume"}
	}
	return []string{"--resume"}
}

// Direct identifies management and noninteractive calls which should keep normal
// shell semantics even when invoked from a terminal.
func Direct(name string, args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "-h" || arg == "--version" || arg == "-V" || arg == "-v" || name == "claude" && (arg == "-p" || arg == "--print") {
			return true
		}
	}
	if len(args) == 0 {
		return false
	}
	if name == "codex" {
		switch args[0] {
		case "exec", "e", "review", "login", "logout", "help", "completion", "doctor", "mcp", "plugin", "update":
			return true
		}
	}
	if name == "claude" {
		switch args[0] {
		case "auth", "doctor", "install", "update", "mcp", "plugin", "logs", "stop", "rm":
			return true
		}
	}
	return false
}
