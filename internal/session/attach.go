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
	fmt.Fprintf(os.Stderr, "\r\nCtrl-%c d detach · Ctrl-%c t take control · repeat prefix to send it literally\r\n", detachPrefix()+64, detachPrefix()+64)
	if m.View {
		fmt.Fprintln(os.Stderr, "Read-only viewer; another terminal may control this session.")
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, old)
	defer fmt.Fprint(os.Stdout, "\x1b[<u\x1b[>4;0m\x1b[?1004l\x1b[?2026l\x1b[0m\x1b[?25h\x1b[?2004l\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l\x1b[?1049l\r\n")
	reader, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		return err
	}
	defer reader.Close()
	defer reader.Cancel()
	done := make(chan error, 2)
	inputDone, outputDone, readDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	input := make(chan []byte, 8)
	stopInput := make(chan struct{})
	go func() {
		defer close(readDone)
		defer close(input)
		buf := make([]byte, 4096)
		for {
			n, e := reader.Read(buf)
			if n > 0 {
				select {
				case input <- append([]byte(nil), buf[:n]...):
				case <-stopInput:
					return
				}
			}
			if e != nil {
				return
			}
		}
	}()
	go func() {
		defer close(inputDone)
		filter := keyFilter{prefix: detachPrefix()}
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for {
			var out []byte
			var actions []string
			select {
			case data, ok := <-input:
				if !ok {
					done <- nil
					return
				}
				out, actions = filter.feed(data)
			case <-ticker.C:
				out = filter.flush()
			case <-stopInput:
				return
			}
			for _, action := range actions {
				if e := send(Message{Type: action}); e != nil {
					done <- e
					return
				}
				if action == "detach" {
					done <- nil
					return
				}
			}
			if len(out) > 0 && !readOnly.Load() {
				if e := send(Message{Type: "input", Data: out}); e != nil {
					done <- e
					return
				}
			}
		}
	}()
	defer func() {
		close(stopInput)
		conn.Close()
		reader.Cancel()
		<-readDone
		<-inputDone
		<-outputDone
	}()
	go func() {
		defer close(outputDone)
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
