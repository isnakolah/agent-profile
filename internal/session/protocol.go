// Package session owns detached provider processes and native terminal attachments.
package session

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"
)

const Protocol = 1
const maxFrame = 4 << 20

// Message is a versioned, length-prefixed local socket frame. Secrets and terminal
// output travel only over the private socket; they are never metadata fields.
type Message struct {
	Version int     `json:"version"`
	Type    string  `json:"type"`
	Data    []byte  `json:"data,omitempty"`
	Spec    *Launch `json:"spec,omitempty"`
	Session *Info   `json:"session,omitempty"`
	Width   int     `json:"width,omitempty"`
	Height  int     `json:"height,omitempty"`
	View    bool    `json:"view,omitempty"`
	Label   string  `json:"label,omitempty"`
	Error   string  `json:"error,omitempty"`
	Code    int     `json:"code,omitempty"`
}
type Launch struct {
	Profile, Provider, Directory, Executable string
	Args, Env                                []string
	Width, Height                            int
}
type Info struct {
	ID         string    `json:"id"`
	Profile    string    `json:"profile"`
	Provider   string    `json:"provider"`
	Directory  string    `json:"directory"`
	State      string    `json:"state"`
	Created    time.Time `json:"created_at"`
	Updated    time.Time `json:"updated_at"`
	Controller string    `json:"controller,omitempty"`
	Viewers    int       `json:"viewers"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	ExitCode   int       `json:"exit_code,omitempty"`
}

func Send(w io.Writer, m Message) error {
	m.Version = Protocol
	b, e := json.Marshal(m)
	if e != nil {
		return e
	}
	if len(b) > maxFrame {
		return errors.New("session frame too large")
	}
	var h [4]byte
	binary.BigEndian.PutUint32(h[:], uint32(len(b)))
	if _, e = w.Write(h[:]); e != nil {
		return e
	}
	_, e = w.Write(b)
	return e
}
func Receive(r io.Reader) (Message, error) {
	var m Message
	var h [4]byte
	if _, e := io.ReadFull(r, h[:]); e != nil {
		return m, e
	}
	n := binary.BigEndian.Uint32(h[:])
	if n == 0 || n > maxFrame {
		return m, errors.New("invalid session frame size")
	}
	b := make([]byte, n)
	if _, e := io.ReadFull(r, b); e != nil {
		return m, e
	}
	if e := json.Unmarshal(b, &m); e != nil {
		return m, e
	}
	if m.Version != Protocol {
		return m, errors.New("session protocol mismatch; use the matching agent-profile version")
	}
	return m, nil
}
func request(socket string, m Message) (Message, error) {
	c, e := net.DialTimeout("unix", socket, time.Second)
	if e != nil {
		return Message{}, e
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	if e = Send(c, m); e != nil {
		return Message{}, e
	}
	r, e := Receive(c)
	if e == nil && r.Error != "" {
		e = errors.New(r.Error)
	}
	return r, e
}
