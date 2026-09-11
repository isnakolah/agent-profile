package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"time"
)

// Item contains display metadata only. Provider terminal output never enters the dashboard.
type Item struct{ ID, Title, Detail, Status, Hint string }
type Loader func() ([]Item, error)
type itemsMsg struct {
	items []Item
	err   error
}
type tickMsg time.Time
type picker struct {
	title, subtitle      string
	items                []Item
	index, width, height int
	choice               Item
	loader               Loader
	err                  error
	loading              bool
}

func (p picker) Init() tea.Cmd {
	if p.loader != nil {
		return p.load()
	}
	return nil
}
func (p picker) load() tea.Cmd {
	return func() tea.Msg { items, e := p.loader(); return itemsMsg{items, e} }
}
func tick() tea.Cmd { return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }
func (p picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = m.Width, m.Height
	case itemsMsg:
		p.loading = false
		p.err = m.err
		if m.err == nil {
			id := ""
			if p.index < len(p.items) {
				id = p.items[p.index].ID
			}
			p.items = m.items
			p.index = 0
			for i, v := range p.items {
				if v.ID == id {
					p.index = i
				}
			}
		}
		return p, tick()
	case tickMsg:
		if p.loader != nil && !p.loading {
			p.loading = true
			return p, p.load()
		}
	case tea.KeyMsg:
		switch m.String() {
		case "q", "esc", "ctrl+c":
			return p, tea.Quit
		case "up", "k":
			if p.index > 0 {
				p.index--
			}
		case "down", "j":
			if p.index+1 < len(p.items) {
				p.index++
			}
		case "home":
			p.index = 0
		case "end":
			p.index = max(0, len(p.items)-1)
		case "enter":
			if p.index < len(p.items) {
				p.choice = p.items[p.index]
				return p, tea.Quit
			}
		case "r":
			if p.loader != nil && !p.loading {
				p.loading = true
				return p, p.load()
			}
		}
	}
	return p, nil
}
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, ansi.Strip(s))
}
func (p picker) View() string {
	w := p.width
	if w == 0 {
		w = 90
	}
	w = max(16, w-6)
	accent := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "25", Dark: "81"})
	muted := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "242", Dark: "245"})
	title := lipgloss.NewStyle().Bold(true)
	var b strings.Builder
	b.WriteString("\n  " + accent.Render("◈") + "  " + title.Render(ansi.Truncate(clean(p.title), w-4, "…")) + "\n")
	b.WriteString("     " + muted.Render(ansi.Truncate(clean(p.subtitle), w-3, "…")) + "\n\n")
	slots := max(1, (p.height-9)/4)
	if p.height == 0 {
		slots = 8
	}
	start := 0
	if p.index >= slots {
		start = p.index - slots + 1
	}
	end := min(len(p.items), start+slots)
	if len(p.items) == 0 {
		b.WriteString("     No sessions yet. Create a profile to begin.\n\n")
	}
	for i := start; i < end; i++ {
		v := p.items[i]
		cursor := "  "
		style := title
		if i == p.index {
			cursor = accent.Render("› ")
			style = style.Foreground(lipgloss.AdaptiveColor{Light: "25", Dark: "81"})
		}
		b.WriteString("  " + cursor + style.Render(ansi.Truncate(clean(v.Title), w-2, "…")) + "\n")
		if v.Detail != "" {
			b.WriteString("     " + muted.Render(ansi.Truncate(clean(v.Detail), w-3, "…")) + "\n")
		}
		meta := v.Status
		if v.Hint != "" {
			if meta != "" {
				meta += "  ·  "
			}
			meta += v.Hint
		}
		if meta != "" {
			b.WriteString("     " + accent.Render(ansi.Truncate(clean(meta), w-3, "…")) + "\n")
		}
		b.WriteString("\n")
	}
	if p.err != nil {
		b.WriteString("     " + ansi.Truncate("Refresh failed: "+clean(p.err.Error()), w-3, "…") + "\n")
	}
	footer := "↑↓ select   enter open   q back"
	if p.loader != nil {
		footer += "   r refresh"
	}
	b.WriteString("  " + muted.Render(ansi.Truncate(footer, w, "…")) + "\n")
	if len(p.items) > slots {
		b.WriteString("  " + muted.Render(fmt.Sprintf("%d–%d of %d", start+1, end, len(p.items))) + "\n")
	}
	return b.String()
}
func Choose(title, subtitle string, items []Item) (Item, error) {
	return ChooseLive(title, subtitle, items, nil)
}
func ChooseLive(title, subtitle string, items []Item, loader Loader) (Item, error) {
	m, e := tea.NewProgram(picker{title: title, subtitle: subtitle, items: items, loader: loader}, tea.WithAltScreen()).Run()
	if e != nil {
		return Item{}, e
	}
	return m.(picker).choice, nil
}

type inputModel struct {
	title, value string
	accepted     bool
}

func (m inputModel) Init() tea.Cmd { return nil }
func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.Type {
		case tea.KeyEnter:
			m.accepted = true
			return m, tea.Quit
		case tea.KeyEsc, tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyBackspace:
			r := []rune(m.value)
			if len(r) > 0 {
				m.value = string(r[:len(r)-1])
			}
		case tea.KeyRunes:
			if len(m.value) < 256 {
				m.value += string(k.Runes)
			}
		}
	}
	return m, nil
}
func (m inputModel) View() string {
	return "\n  " + m.title + "\n\n  › " + m.value + "▏\n\n  enter confirm · esc cancel\n"
}
func Input(title string) (string, error) {
	m, e := tea.NewProgram(inputModel{title: title}).Run()
	if e != nil {
		return "", e
	}
	v := m.(inputModel)
	if !v.accepted {
		return "", nil
	}
	return strings.TrimSpace(v.value), nil
}
