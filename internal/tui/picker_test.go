package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestPickerNavigationAndNarrowLayout(t *testing.T) {
	p := picker{title: "Agent Profile", subtitle: "Sessions", width: 40, height: 20, items: []Item{{ID: "1", Title: "Work", Detail: strings.Repeat("very long directory ", 10)}, {ID: "2", Title: "Personal", Status: "attached"}}}
	m, _ := p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p = m.(picker)
	if p.index != 1 {
		t.Fatal("selection failed")
	}
	for _, line := range strings.Split(p.View(), "\n") {
		if ansi.StringWidth(line) > 40 {
			t.Errorf("overflow: %s", line)
		}
	}
	m, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.(picker).choice.ID != "2" {
		t.Fatal("wrong selection")
	}
}
func TestRefreshPreservesSelectionAndEscapesMetadata(t *testing.T) {
	p := picker{items: []Item{{ID: "a"}, {ID: "b"}}, index: 1}
	m, _ := p.Update(itemsMsg{items: []Item{{ID: "b", Title: "bad\x1b[2Jtitle"}, {ID: "a"}}})
	p = m.(picker)
	if p.index != 0 {
		t.Fatal("selection moved")
	}
	if clean("a\x1b[2Jb\n") != "ab " {
		t.Fatal("unsafe terminal metadata")
	}
}
