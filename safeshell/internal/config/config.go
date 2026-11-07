// Package config loads the user's settings from ~/.config/safeshell/config.json.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"safeshell/internal/ai"
)

// Config holds every setting. The zero value is valid: AI off.
type Config struct {
	AI struct {
		Mode  ai.Mode `json:"mode"` // off, local, cloud, hybrid
		Local struct {
			URL   string `json:"url"`
			Model string `json:"model"`
		} `json:"local"`
		Cloud struct {
			Model string `json:"model"`
		} `json:"cloud"` // the key is read from ANTHROPIC_API_KEY, never stored
	} `json:"ai"`
}

// Path is $XDG_CONFIG_HOME/safeshell/config.json (or ~/.config/...).
func Path() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "safeshell", "config.json")
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config", "safeshell", "config.json")
}

// Load reads path. A missing file gives defaults; SAFESHELL_AI overrides the mode.
func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return c, err
	default:
		if err := json.Unmarshal(b, &c); err != nil {
			return Config{}, errors.New(path + ": " + err.Error())
		}
	}
	if m := os.Getenv("SAFESHELL_AI"); m != "" {
		c.AI.Mode = ai.Mode(m)
	}
	switch c.AI.Mode {
	case "":
		c.AI.Mode = ai.Off
	case ai.Off, ai.Local, ai.Cloud, ai.Hybrid:
	default:
		return Config{}, errors.New("ai.mode must be off, local, cloud or hybrid, not " + string(c.AI.Mode))
	}
	if c.AI.Local.Model == "" {
		c.AI.Local.Model = "qwen2.5-coder:3b"
	}
	return c, nil
}
