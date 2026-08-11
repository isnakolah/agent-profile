package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Row struct {
	Profile string
	Codex   string
	Claude  string
}

type model struct {
	rows  []Row
	index int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.index > 0 {
				m.index--
			}
		case "down", "j":
			if m.index < len(m.rows)-1 {
				m.index++
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("Agent Profile\n\n")
	if len(m.rows) == 0 {
		b.WriteString("No profiles. Create one with: agent-profile profile create NAME\n")
	} else {
		b.WriteString("PROFILE                 CODEX                 CLAUDE\n")
		b.WriteString("-----------------------------------------------------------\n")
		for i, row := range m.rows {
			cursor := "  "
			if i == m.index {
				cursor = "> "
			}
			fmt.Fprintf(&b, "%s%-22s %-21s %s\n", cursor, row.Profile, row.Codex, row.Claude)
		}
	}
	b.WriteString("\n↑/↓ select   q quit\n")
	return b.String()
}

func Run(rows []Row) error {
	_, err := tea.NewProgram(model{rows: rows}).Run()
	return err
}
