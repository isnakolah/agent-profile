package app

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func notifyCommand(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: notify NAME MESSAGE")
	}
	message := strings.Join(args[1:], " ")
	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.close()
	if _, err := store.profile(args[0]); err != nil {
		return err
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(message)))
	dedupPath := filepath.Join(store.eventsDir, args[0]+"-notification.json")
	var previous struct {
		Key string    `json:"key"`
		At  time.Time `json:"at"`
	}
	if b, readErr := os.ReadFile(dedupPath); readErr == nil {
		_ = json.Unmarshal(b, &previous)
	}
	if !notificationDue(previous.Key, previous.At, key, time.Now()) {
		return nil
	}

	if runtime.GOOS == "darwin" {
		if err := exec.Command("osascript", "-e", fmt.Sprintf("display notification %q with title %q", message, "agent-profile "+args[0])).Run(); err != nil {
			return err
		}
		if err := atomicJSON(dedupPath, map[string]any{"key": key, "at": time.Now().UTC()}, 0600); err != nil {
			return err
		}
		return appendEvent(store, Event{At: time.Now().UTC(), Action: "notify", Profile: args[0], Detail: "native notification"})
	}
	if _, err := exec.LookPath("notify-send"); err != nil {
		return fmt.Errorf("native notification unavailable: %w", err)
	}
	if err := exec.Command("notify-send", "agent-profile "+args[0], message).Run(); err != nil {
		return err
	}
	if err := atomicJSON(dedupPath, map[string]any{"key": key, "at": time.Now().UTC()}, 0600); err != nil {
		return err
	}
	return appendEvent(store, Event{At: time.Now().UTC(), Action: "notify", Profile: args[0], Detail: "native notification"})
}

func notificationDue(previousKey string, previousAt time.Time, key string, now time.Time) bool {
	return previousKey != key || now.Sub(previousAt) >= time.Hour
}
