package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDashboardViewShowsProviderState(t *testing.T) {
	m := model{rows: []Row{{Profile: "work", Codex: "available", Claude: "unavailable"}}}
	view := m.View()
	for _, want := range []string{"work", "available", "unavailable", "q quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("dashboard missing %q: %s", want, view)
		}
	}
}

func TestDashboardKeyboardSelection(t *testing.T) {
	m := model{rows: []Row{{Profile: "one"}, {Profile: "two"}}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.(model).index != 1 {
		t.Fatalf("down did not select second profile: %#v", updated)
	}
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updated.(model).index != 0 {
		t.Fatalf("up did not select first profile: %#v", updated)
	}
}
