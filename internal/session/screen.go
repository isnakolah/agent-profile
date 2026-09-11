package session

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	vt "github.com/charmbracelet/x/vt"
	"sort"
	"strings"
)

// screen retains a bounded terminal model, never an on-disk transcript.
// All methods are called under the worker mutex.
type screen struct {
	vt      *vt.Emulator
	modes   map[int]bool
	visible bool
}

func newScreen(w, h int) *screen {
	s := &screen{vt: vt.NewEmulator(w, h), modes: map[int]bool{}, visible: true}
	s.vt.SetScrollbackSize(1000)
	s.vt.SetCallbacks(vt.Callbacks{CursorVisibility: func(v bool) { s.visible = v }, EnableMode: func(m ansi.Mode) {
		if d, ok := m.(ansi.DECMode); ok {
			s.modes[int(d)] = true
		}
	}, DisableMode: func(m ansi.Mode) {
		if d, ok := m.(ansi.DECMode); ok {
			s.modes[int(d)] = false
		}
	}})
	return s
}
func (s *screen) snapshot() []byte {
	var b strings.Builder
	b.WriteString("\x1b[0m\x1b[?25l")
	if s.vt.IsAltScreen() {
		b.WriteString("\x1b[?1049h")
	} else {
		b.WriteString("\x1b[?1049l")
	}
	b.WriteString("\x1b[H\x1b[2J")
	if !s.vt.IsAltScreen() {
		for _, line := range s.vt.Scrollback().Lines() {
			b.WriteString(line.Render())
			b.WriteString("\r\n")
		}
	}
	b.WriteString(strings.ReplaceAll(s.vt.Render(), "\n", "\r\n"))
	keys := []int{}
	for n := range s.modes {
		keys = append(keys, n)
	}
	sort.Ints(keys)
	for _, n := range keys {
		if n == 1049 || n == 47 || n == 1047 || n == 25 {
			continue
		}
		suffix := "l"
		if s.modes[n] {
			suffix = "h"
		}
		fmt.Fprintf(&b, "\x1b[?%d%s", n, suffix)
	}
	p := s.vt.CursorPosition()
	fmt.Fprintf(&b, "\x1b[%d;%dH", p.Y+1, p.X+1)
	if s.visible {
		b.WriteString("\x1b[?25h")
	}
	return []byte(b.String())
}
