package session

import (
	"errors"
	"fmt"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ExitError preserves the provider's exit status for scripts and shell callers.
type ExitError struct{ Code int }

func (e ExitError) Error() string             { return fmt.Sprintf("provider exited with status %d", e.Code) }
func Attach(root, id string, view bool) error { return attach(root, id, view, false) }
func AttachControl(root, id string) error     { return attach(root, id, false, true) }
func attach(root, id string, view, takeover bool) error {
	if !validID(id) {
		return errors.New("invalid session ID")
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return errors.New("attach requires a terminal")
	}
	conn, err := net.DialTimeout("unix", socketPath(root, id), time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	width, height, _ := term.GetSize(fd)
	var sendMu sync.Mutex
	send := func(m Message) error { sendMu.Lock(); defer sendMu.Unlock(); return Send(conn, m) }
	label := os.Getenv("TERM_PROGRAM")
	if label == "" {
		label = os.Getenv("TERM")
	}
	label = fmt.Sprintf("%s · pid %d", label, os.Getpid())
	if err = send(Message{Type: "attach", View: view, Label: label, Width: width, Height: height}); err != nil {
		return err
	}
	m, err := Receive(conn)
	if err != nil {
		return err
	}
	if m.Error != "" {
		return errors.New(m.Error)
	}
	if takeover {
		if err = send(Message{Type: "takeover"}); err != nil {
			return err
		}
	}
	var readOnly atomic.Bool
	readOnly.Store(m.View)
	fmt.Fprintln(os.Stderr, "\r\nCtrl-] d detach · Ctrl-] t take control · Ctrl-] Ctrl-] send prefix")
	if m.View {
		fmt.Fprintln(os.Stderr, "Read-only viewer; another terminal may control this session.")
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, old)
	defer fmt.Fprint(os.Stdout, "\x1b[0m\x1b[?25h\x1b[?2004l\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l\x1b[?1049l\r\n")
	reader, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		return err
	}
	defer reader.Close()
	defer reader.Cancel()
	done := make(chan error, 2)
	go func() {
		buf := make([]byte, 4096)
		prefix := false
		for {
			n, e := reader.Read(buf)
			if e != nil {
				done <- nil
				return
			}
			out := []byte{}
			for _, b := range buf[:n] {
				if prefix {
					prefix = false
					switch b {
					case 'd':
						_ = send(Message{Type: "detach"})
						done <- nil
						return
					case 't':
						_ = send(Message{Type: "takeover"})
						continue
					case 29:
						out = append(out, 29)
						continue
					default:
						out = append(out, 29, b)
						continue
					}
				}
				if b == 29 {
					prefix = true
				} else {
					out = append(out, b)
				}
			}
			if len(out) > 0 && !readOnly.Load() {
				if e = send(Message{Type: "input", Data: out}); e != nil {
					done <- e
					return
				}
			}
		}
	}()
	go func() {
		for {
			m, e := Receive(conn)
			if e != nil {
				done <- e
				return
			}
			switch m.Type {
			case "output":
				if _, e = os.Stdout.Write(m.Data); e != nil {
					done <- e
					return
				}
			case "role":
				readOnly.Store(m.View)
			case "exit":
				if m.Code != 0 {
					done <- ExitError{m.Code}
				} else {
					done <- nil
				}
				return
			case "error":
				done <- errors.New(m.Error)
				return
			}
		}
	}()
	resize := make(chan os.Signal, 1)
	signal.Notify(resize, syscall.SIGWINCH)
	defer signal.Stop(resize)
	closed := make(chan struct{})
	defer close(closed)
	go func() {
		for {
			select {
			case <-resize:
				w, h, e := term.GetSize(fd)
				if e == nil {
					_ = send(Message{Type: "resize", Width: w, Height: h})
				}
			case <-closed:
				return
			}
		}
	}()
	return <-done
}
