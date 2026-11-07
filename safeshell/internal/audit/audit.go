// Package audit keeps an append-only record of what SafeShell decided.
package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry is one decision. Outcome is "allowed", "cancelled", "ran", or
// "replaced" (a safer option was run instead of the original).
type Entry struct {
	Time     time.Time `json:"time"`
	Line     string    `json:"line"`
	Run      string    `json:"run,omitempty"`
	Level    string    `json:"level"`
	Decision string    `json:"decision"`
	Outcome  string    `json:"outcome"`
}

// DefaultPath is $XDG_STATE_HOME/safeshell/audit.jsonl (or ~/.local/state/...).
func DefaultPath() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "safeshell", "audit.jsonl")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".local", "state", "safeshell", "audit.jsonl")
}

// Append writes one line. Failure to log must never stop a command, so errors
// are returned for the caller to ignore or report.
func Append(path string, e Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(e)
}
