package session

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/state"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == "_session-worker" {
		if e := Serve(os.Args[2], os.Args[3]); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestProviderFixture(t *testing.T) {
	if os.Getenv("AP_FIXTURE") != "1" {
		return
	}
	fmt.Print("\x1b[?1049h\x1b[2J\x1b[Hready 世界\r\n")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		s := scanner.Text()
		if s == "exit" {
			os.Exit(7)
		}
		fmt.Printf("received:%s\r\n", s)
	}
	os.Exit(0)
}
func startFixture(t *testing.T) (string, Info) {
	t.Helper()
	root := t.TempDir()
	exe, _ := os.Executable()
	info, e := Start(root, exe, Launch{Profile: "work", Provider: "codex", Directory: root, Executable: exe, Args: []string{"-test.run=^TestProviderFixture$"}, Env: append(os.Environ(), "AP_FIXTURE=1"), Width: 80, Height: 24})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = Stop(root, info.ID) })
	return root, info
}
func connect(t *testing.T, root, id, label string, view bool) net.Conn {
	t.Helper()
	c, e := net.Dial("unix", socketPath(root, id))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if e = Send(c, Message{Type: "attach", Label: label, View: view, Width: 90, Height: 30}); e != nil {
		t.Fatal(e)
	}
	m, e := Receive(c)
	if e != nil || m.Type != "attached" {
		t.Fatalf("attach: %+v %v", m, e)
	}
	return c
}
func until(t *testing.T, c net.Conn, kind, text string) Message {
	t.Helper()
	for range 100 {
		m, e := Receive(c)
		if e != nil {
			t.Fatalf("waiting for %s/%s: %v", kind, text, e)
		}
		if m.Type == kind && (text == "" || bytes.Contains(m.Data, []byte(text))) {
			return m
		}
	}
	t.Fatal("message missing")
	return Message{}
}
func TestDetachReattachViewAndControl(t *testing.T) {
	root, info := startFixture(t)
	a := connect(t, root, info.ID, "first", false)
	until(t, a, "output", "ready")
	v := connect(t, root, info.ID, "viewer", true)
	until(t, v, "output", "ready")
	_ = Send(v, Message{Type: "input", Data: []byte("forbidden\n")})
	_ = Send(a, Message{Type: "input", Data: []byte("allowed\n")})
	m := until(t, a, "output", "received:allowed")
	if bytes.Contains(m.Data, []byte("received:forbidden")) {
		t.Fatal("viewer wrote to process")
	}
	all, e := List(root)
	if e != nil || len(all) != 1 || all[0].Viewers != 1 || all[0].Controller != "first" {
		t.Fatalf("status: %+v %v", all, e)
	}
	_ = Send(v, Message{Type: "takeover"})
	until(t, v, "role", "")
	until(t, a, "role", "")
	_ = Send(v, Message{Type: "resize", Width: 100, Height: 40})
	_ = Send(v, Message{Type: "input", Data: []byte("second\n")})
	until(t, v, "output", "received:second")
	a.Close()
	v.Close()
	time.Sleep(100 * time.Millisecond)
	b := connect(t, root, info.ID, "reattached", false)
	until(t, b, "output", "received:second")
	all, e = List(root)
	if e != nil || all[0].State != "running" || all[0].Controller != "reattached" {
		t.Fatalf("reattach: %+v %v", all, e)
	}
	_ = Send(b, Message{Type: "input", Data: []byte("exit\n")})
	m = until(t, b, "exit", "")
	if m.Code != 7 {
		t.Fatalf("exit=%d", m.Code)
	}
}
func TestInterruptedMetadataAndNoSecrets(t *testing.T) {
	root := t.TempDir()
	id := strings.Repeat("ab", 12)
	e := state.JSON(metadata(root, id), Info{ID: id, Profile: "p", State: "running"})
	if e != nil {
		t.Fatal(e)
	}
	all, e := List(root)
	if e != nil || all[0].State != "interrupted" {
		t.Fatalf("%+v %v", all, e)
	}
	b, _ := json.Marshal(all[0])
	for _, key := range []string{"Env", "Args", "token", "prompt"} {
		if bytes.Contains(b, []byte(key)) {
			t.Fatal("sensitive metadata field")
		}
	}
}
func TestProtocolRejectsOversizedAndWrongVersion(t *testing.T) {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, uint32(maxFrame+1))
	if _, e := Receive(&b); e == nil {
		t.Fatal("oversized frame accepted")
	}
	b.Reset()
	data := []byte(`{"version":99,"type":"status"}`)
	_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
	b.Write(data)
	if _, e := Receive(&b); e == nil {
		t.Fatal("wrong protocol accepted")
	}
}
func TestStopAndRemove(t *testing.T) {
	root, s := startFixture(t)
	if e := Remove(root, s.ID); e == nil {
		t.Fatal("removed running session")
	}
	if e := Stop(root, s.ID); e != nil {
		t.Fatal(e)
	}
	for range 100 {
		all, e := List(root)
		if e != nil {
			t.Fatal(e)
		}
		if all[0].State != "running" {
			if e = Remove(root, s.ID); e != nil {
				t.Fatal(e)
			}
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("stop did not finish")
}

func TestDetachPrefixWithNativeKeyboardProtocols(t *testing.T) {
	for _, seq := range []string{"\x1dd", "\x1b[93;5ud", "\x1b[93;5u\x1b[100u"} {
		f := keyFilter{prefix: 29}
		var actions []string
		for _, b := range []byte(seq) {
			out, a := f.feed([]byte{b})
			if len(out) > 0 {
				t.Fatalf("prefix leaked: %q", out)
			}
			actions = append(actions, a...)
		}
		if len(actions) != 1 || actions[0] != "detach" {
			t.Fatalf("did not detach: %q %+v", seq, actions)
		}
	}
	f := keyFilter{prefix: 29}
	out, actions := f.feed([]byte("hello\x1b[A"))
	if string(out) != "hello\x1b[A" || len(actions) != 0 {
		t.Fatal("provider input changed")
	}
}

func TestPastedPrefixIsNotInterpreted(t *testing.T) {
	f := keyFilter{prefix: 29}
	data := []byte("\x1b[200~hello\x1dd\x1b[201~")
	out, actions := f.feed(data)
	if len(actions) != 0 || !bytes.Equal(out, data) {
		t.Fatal("paste interpreted as shortcut")
	}
}
