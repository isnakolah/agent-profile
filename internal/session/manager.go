package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/isnakolah/agent-profile/internal/state"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

func validID(id string) bool       { b, e := hex.DecodeString(id); return e == nil && len(b) == 12 }
func directory(root string) string { return filepath.Join(root, "sessions") }
func socketPath(root, id string) string {
	// Darwin's sockaddr_un is only 104 bytes. Keep sockets in an owned, private
	// temporary directory; metadata stays in the durable profile root.
	return filepath.Join("/tmp", fmt.Sprintf("agent-profile-%d", os.Getuid()), id+".sock")
}
func metadata(root, id string) string { return filepath.Join(directory(root), id+".json") }
func Start(root, workerExecutable string, spec Launch) (Info, error) {
	if err := state.PrivateDir(directory(root)); err != nil {
		return Info{}, err
	}
	b := make([]byte, 12)
	if _, e := rand.Read(b); e != nil {
		return Info{}, e
	}
	id := hex.EncodeToString(b)
	sock := socketPath(root, id)
	if e := state.PrivateDir(filepath.Dir(sock)); e != nil {
		return Info{}, e
	}
	cmd := exec.Command(workerExecutable, "_session-worker", root, id)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if e := cmd.Start(); e != nil {
		return Info{}, e
	}
	defer cmd.Process.Release()
	var c net.Conn
	var err error
	for range 100 {
		c, err = net.DialTimeout("unix", sock, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		return Info{}, fmt.Errorf("session worker did not start: %w", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(15 * time.Second))
	if err = Send(c, Message{Type: "start", Spec: &spec}); err != nil {
		return Info{}, err
	}
	reply, err := Receive(c)
	if err != nil {
		return Info{}, err
	}
	if reply.Error != "" {
		return Info{}, errors.New(reply.Error)
	}
	if reply.Session == nil {
		return Info{}, errors.New("missing session response")
	}
	return *reply.Session, nil
}
func List(root string) ([]Info, error) {
	entries, err := os.ReadDir(directory(root))
	if os.IsNotExist(err) {
		return []Info{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Info{}
	for _, entry := range entries {
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !validID(id) || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, e := os.ReadFile(metadata(root, id))
		if e != nil {
			return nil, e
		}
		var info Info
		if e = json.Unmarshal(b, &info); e != nil {
			return nil, fmt.Errorf("session %s metadata: %w", id, e)
		}
		if info.ID != id {
			return nil, errors.New("session metadata identity mismatch")
		}
		if info.State == "running" {
			r, e := request(socketPath(root, id), Message{Type: "status"})
			if e == nil && r.Session != nil {
				info = *r.Session
			} else {
				info.State = "interrupted"
				info.Controller = ""
				info.Viewers = 0
			}
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}
func Find(root, id string) (Info, error) {
	all, e := List(root)
	if e != nil {
		return Info{}, e
	}
	for _, s := range all {
		if s.ID == id {
			return s, nil
		}
	}
	return Info{}, fmt.Errorf("session %q not found", id)
}
func Stop(root, id string) error {
	if !validID(id) {
		return errors.New("invalid session ID")
	}
	_, e := request(socketPath(root, id), Message{Type: "stop"})
	return e
}
func Remove(root, id string) error {
	s, e := Find(root, id)
	if e != nil {
		return e
	}
	if s.State == "running" {
		return errors.New("stop the session before removing its metadata")
	}
	return os.Remove(metadata(root, id))
}
