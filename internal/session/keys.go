package session

import (
	"os"
	"strconv"
	"strings"
)

// keyFilter recognizes our prefix in legacy and CSI-u keyboards. Other bytes,
// including paste and provider-specific key sequences, pass through untouched.
type keyFilter struct {
	prefix  byte
	pending bool
	escape  []byte
	pasted  bool
}

func detachPrefix() byte {
	s := strings.ToLower(os.Getenv("AGENT_PROFILE_DETACH_KEY"))
	if strings.HasPrefix(s, "ctrl-") && len(s) == 6 {
		b := s[5]
		if b >= 'a' && b <= 'z' {
			return b - 'a' + 1
		}
		if b >= '[' && b <= '_' {
			return b - 64
		}
	}
	return 29
}
func (f *keyFilter) token(raw []byte) (out []byte, action string) {
	if string(raw) == "\x1b[200~" {
		f.pasted = true
		return raw, ""
	}
	if string(raw) == "\x1b[201~" {
		f.pasted = false
		return raw, ""
	}
	if f.pasted {
		return raw, ""
	}
	key := byte(0)
	if len(raw) == 1 {
		key = raw[0]
	} else if strings.HasPrefix(string(raw), "\x1b[") && raw[len(raw)-1] == 'u' {
		parts := strings.Split(string(raw[2:len(raw)-1]), ";")
		n, _ := strconv.Atoi(strings.Split(parts[0], ":")[0])
		mod := 1
		if len(parts) > 1 {
			mod, _ = strconv.Atoi(strings.Split(parts[1], ":")[0])
			if strings.HasSuffix(parts[1], ":3") {
				return raw, ""
			}
		}
		if mod == 5 && n >= 64 && n <= 127 {
			key = byte(n) & 31
		} else if mod == 1 && n < 128 {
			key = byte(n)
		}
	}
	if f.pending {
		f.pending = false
		if key == 'd' {
			return nil, "detach"
		}
		if key == 't' {
			return nil, "takeover"
		}
		if key == f.prefix {
			return raw, ""
		}
		return append([]byte{f.prefix}, raw...), ""
	}
	if key == f.prefix {
		f.pending = true
		return nil, ""
	}
	return raw, ""
}
func (f *keyFilter) feed(data []byte) (out []byte, actions []string) {
	for _, b := range data {
		if len(f.escape) > 0 {
			f.escape = append(f.escape, b)
			if len(f.escape) == 2 && b == '[' {
				continue
			}
			if len(f.escape) > 2 && b >= 0x30 && b <= 0x3f && len(f.escape) < 64 {
				continue
			}
			v, a := f.token(f.escape)
			out = append(out, v...)
			if a != "" {
				actions = append(actions, a)
			}
			f.escape = nil
			continue
		}
		if b == 27 {
			f.escape = []byte{b}
			continue
		}
		v, a := f.token([]byte{b})
		out = append(out, v...)
		if a != "" {
			actions = append(actions, a)
		}
	}
	return
}
func (f *keyFilter) flush() []byte { b := f.escape; f.escape = nil; return b }
