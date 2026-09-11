package provider

import (
	"reflect"
	"strings"
	"testing"
)

func TestEnvironmentReplacesAuthentication(t *testing.T) {
	env := Environment([]string{"PATH=/bin", "CODEX_HOME=/other", "OPENAI_API_KEY=wrong", "ANTHROPIC_AUTH_TOKEN=wrong", "CLAUDE_CODE_USE_BEDROCK=1", "TERM=xterm"}, "codex", "/profiles/work/codex", "selected")
	joined := strings.Join(env, "\n")
	for _, bad := range []string{"wrong", "/other", "BEDROCK"} {
		if strings.Contains(joined, bad) {
			t.Fatalf("inherited auth escaped: %s", bad)
		}
	}
	if !strings.Contains(joined, "OPENAI_API_KEY=selected") || !strings.Contains(joined, "CODEX_HOME=/profiles/work/codex") || !strings.Contains(joined, "PATH=/bin") {
		t.Fatal("missing explicit environment")
	}
}
func TestArgumentsPreserveLiteralProviderParameters(t *testing.T) {
	args := []string{"--model", "value with spaces", "$(touch /tmp/never)", "--", "-prompt"}
	for _, p := range []string{"claude", "codex"} {
		got := Arguments(p, "api-key", args)
		if !reflect.DeepEqual(got[len(got)-len(args):], args) {
			t.Fatal("arguments altered")
		}
	}
}
