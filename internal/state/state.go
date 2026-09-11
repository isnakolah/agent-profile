// Package state provides private, atomic metadata storage and process locks.
package state

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func Root() (string, error) {
	if p := os.Getenv("AGENT_PROFILE_HOME"); p != "" {
		return filepath.Abs(p)
	}
	p, e := os.UserConfigDir()
	return filepath.Join(p, "agent-profile"), e
}
func PrivateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe directory: %s", path)
	}
	return os.Chmod(path, 0700)
}
func Lock(path string) (*os.File, error) {
	if err := PrivateDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	if err = unix.Flock(fd, unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func Unlock(f *os.File) {
	if f != nil {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}
}
func JSON(path string, value any) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	return Write(path, append(b, '\n'), 0600)
}
func Write(path string, b []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
