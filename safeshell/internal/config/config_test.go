package config

import (
	"os"
	"path/filepath"
	"testing"

	"safeshell/internal/ai"
)

func write(t *testing.T, s string) string {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte(s), 0o600)
	return p
}

func TestDefaultsWhenMissing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if err != nil || c.AI.Mode != ai.Off || c.AI.Local.Model == "" {
		t.Errorf("%+v %v", c, err)
	}
}

func TestLoadsValuesAndEnvOverridesMode(t *testing.T) {
	p := write(t, `{"ai":{"mode":"hybrid","local":{"model":"llama3.2:3b","url":"http://h:1"},"cloud":{"model":"claude-sonnet-5-5"}}}`)
	c, err := Load(p)
	if err != nil || c.AI.Mode != ai.Hybrid || c.AI.Local.Model != "llama3.2:3b" || c.AI.Cloud.Model != "claude-sonnet-5-5" {
		t.Fatalf("%+v %v", c, err)
	}
	t.Setenv("SAFESHELL_AI", "off")
	if c, _ := Load(p); c.AI.Mode != ai.Off {
		t.Error("env override")
	}
}

func TestRejectsBadModeAndBadJSON(t *testing.T) {
	if _, err := Load(write(t, `{"ai":{"mode":"always"}}`)); err == nil {
		t.Error("bad mode")
	}
	if _, err := Load(write(t, `{`)); err == nil {
		t.Error("bad json")
	}
}

func TestPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/c")
	if Path() != "/c/safeshell/config.json" {
		t.Error(Path())
	}
}
