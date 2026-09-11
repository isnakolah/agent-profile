package session

import (
	"errors"
	"github.com/creack/pty"
	"github.com/isnakolah/agent-profile/internal/state"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type client struct {
	conn          net.Conn
	queue         chan Message
	label         string
	width, height int
	done          chan struct{}
}
type worker struct {
	mu         sync.Mutex
	root       string
	info       Info
	terminal   *os.File
	command    *exec.Cmd
	screen     *screen
	clients    map[*client]bool
	controller *client
	controlled atomic.Bool
	finished   chan struct{}
	input      chan []byte
}

// Serve runs in a detached process. The coordinator never owns its PTY.
func Serve(root, id string) error {
	if !validID(id) {
		return errors.New("invalid worker identity")
	}
	listener, err := net.Listen("unix", socketPath(root, id))
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(socketPath(root, id))
	_ = os.Chmod(socketPath(root, id), 0600)
	_ = listener.(*net.UnixListener).SetDeadline(time.Now().Add(15 * time.Second))
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	m, err := Receive(conn)
	if err != nil {
		conn.Close()
		return err
	}
	if m.Type != "start" || m.Spec == nil {
		conn.Close()
		return errors.New("expected launch request")
	}
	spec := m.Spec
	if spec.Width < 1 {
		spec.Width = 80
	}
	if spec.Height < 1 {
		spec.Height = 24
	}
	if spec.Width > 500 || spec.Height > 300 {
		conn.Close()
		return errors.New("terminal dimensions exceed limits")
	}
	w := &worker{root: root, clients: map[*client]bool{}, finished: make(chan struct{}), input: make(chan []byte, 64), info: Info{ID: id, Profile: spec.Profile, Provider: spec.Provider, Directory: spec.Directory, State: "running", Created: time.Now().UTC(), Updated: time.Now().UTC(), Width: spec.Width, Height: spec.Height}}
	w.command = exec.Command(spec.Executable, spec.Args...)
	w.command.Env = spec.Env
	w.command.Dir = spec.Directory
	w.terminal, err = pty.StartWithSize(w.command, &pty.Winsize{Cols: uint16(spec.Width), Rows: uint16(spec.Height)})
	// No credentials or prompt arguments remain in session metadata.
	spec = nil
	m.Spec = nil
	if err != nil {
		_ = Send(conn, Message{Type: "error", Error: err.Error()})
		conn.Close()
		return err
	}
	defer w.terminal.Close()
	if err = state.JSON(metadata(root, id), w.info); err != nil {
		_ = w.command.Process.Kill()
		_ = w.command.Wait()
		conn.Close()
		return err
	}
	w.screen = newScreen(w.info.Width, w.info.Height)
	go func() {
		for {
			select {
			case b := <-w.input:
				if _, e := w.terminal.Write(b); e != nil {
					return
				}
			case <-w.finished:
				return
			}
		}
	}()
	// Drain emulator replies continuously, including while no terminal is attached.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, e := w.screen.vt.Read(buf)
			if n > 0 && !w.controlled.Load() {
				select {
				case w.input <- append([]byte(nil), buf[:n]...):
				default:
				}
			}
			if e != nil {
				return
			}
		}
	}()
	_ = Send(conn, Message{Type: "started", Session: &w.info})
	conn.Close()
	_ = listener.(*net.UnixListener).SetDeadline(time.Time{})
	go w.output()
	go func() {
		err := w.command.Wait()
		code := 0
		if err != nil {
			code = 1
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				code = exit.ExitCode()
			}
		}
		// PTY output drains before terminal exit is broadcast.
		select {
		case <-w.finished:
		case <-time.After(time.Second):
		}
		w.mu.Lock()
		w.info.State = "exited"
		w.info.ExitCode = code
		w.info.Updated = time.Now().UTC()
		_ = state.JSON(metadata(root, id), w.info)
		for c := range w.clients {
			w.enqueue(c, Message{Type: "exit", Code: code})
		}
		w.mu.Unlock()
		time.Sleep(150 * time.Millisecond)
		_ = listener.Close()
	}()
	for {
		c, e := listener.Accept()
		if e != nil {
			break
		}
		go w.handle(c)
	}
	w.mu.Lock()
	for c := range w.clients {
		_ = c.conn.Close()
	}
	w.mu.Unlock()
	if closer, ok := w.screen.vt.InputPipe().(io.Closer); ok {
		_ = closer.Close()
	}
	return nil
}
func (w *worker) output() {
	defer close(w.finished)
	buf := make([]byte, 32*1024)
	for {
		n, e := w.terminal.Read(buf)
		if n > 0 {
			w.mu.Lock()
			_, _ = w.screen.vt.Write(buf[:n])
			for c := range w.clients {
				w.enqueue(c, Message{Type: "output", Data: append([]byte(nil), buf[:n]...)})
			}
			w.mu.Unlock()
		}
		if e != nil {
			return
		}
	}
}
func (w *worker) enqueue(c *client, m Message) {
	select {
	case c.queue <- m:
	default:
		_ = c.conn.Close()
	}
}
func (w *worker) status() Info {
	s := w.info
	s.Viewers = len(w.clients)
	if w.controller != nil {
		s.Controller = w.controller.label
		s.Viewers--
	}
	return s
}
func (w *worker) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	m, e := Receive(conn)
	if e != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	w.mu.Lock()
	switch m.Type {
	case "status":
		s := w.status()
		w.mu.Unlock()
		_ = Send(conn, Message{Type: "status", Session: &s})
		return
	case "stop":
		w.mu.Unlock()
		_ = syscall.Kill(-w.command.Process.Pid, syscall.SIGTERM)
		_ = Send(conn, Message{Type: "stopping"})
		go func() {
			time.Sleep(3 * time.Second)
			w.mu.Lock()
			running := w.info.State == "running"
			w.mu.Unlock()
			if running {
				_ = syscall.Kill(-w.command.Process.Pid, syscall.SIGKILL)
			}
		}()
		return
	case "attach":
	default:
		w.mu.Unlock()
		_ = Send(conn, Message{Type: "error", Error: "unknown operation"})
		return
	}
	if w.info.State != "running" {
		w.mu.Unlock()
		_ = Send(conn, Message{Type: "error", Error: "session has exited"})
		return
	}
	c := &client{conn: conn, queue: make(chan Message, 64), label: m.Label, width: m.Width, height: m.Height, done: make(chan struct{})}
	if c.label == "" {
		c.label = "terminal"
	}
	w.clients[c] = true
	if !m.View && w.controller == nil {
		w.controller = c
		w.controlled.Store(true)
		w.resize(c)
	}
	s := w.status()
	w.enqueue(c, Message{Type: "attached", Session: &s, View: w.controller != c})
	w.enqueue(c, Message{Type: "output", Data: w.screen.snapshot()})
	w.mu.Unlock()
	go func() {
		for {
			select {
			case msg := <-c.queue:
				_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if Send(conn, msg) != nil {
					conn.Close()
					return
				}
			case <-c.done:
				return
			}
		}
	}()
	defer func() {
		w.mu.Lock()
		delete(w.clients, c)
		close(c.done)
		if w.controller == c {
			w.controller = nil
			w.controlled.Store(false)
		}
		w.mu.Unlock()
	}()
	for {
		m, e = Receive(conn)
		if e != nil {
			return
		}
		w.mu.Lock()
		switch m.Type {
		case "input":
			if w.controller == c {
				if len(m.Data) <= 32768 {
					select {
					case w.input <- m.Data:
					default:
						_ = c.conn.Close()
					}
				}
			}
		case "resize":
			c.width, c.height = m.Width, m.Height
			if w.controller == c {
				w.resize(c)
			}
		case "takeover":
			old := w.controller
			w.controller = c
			w.controlled.Store(true)
			w.resize(c)
			if old != nil && old != c {
				w.enqueue(old, Message{Type: "role", View: true})
			}
			w.enqueue(c, Message{Type: "role", View: false})
		case "detach":
			w.mu.Unlock()
			return
		}
		w.mu.Unlock()
	}
}
func (w *worker) resize(c *client) {
	if c.width < 1 || c.height < 1 || c.width > 500 || c.height > 300 {
		return
	}
	w.info.Width, w.info.Height = c.width, c.height
	_ = pty.Setsize(w.terminal, &pty.Winsize{Cols: uint16(c.width), Rows: uint16(c.height)})
	w.screen.vt.Resize(c.width, c.height)
}
