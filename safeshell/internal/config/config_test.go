package config

import (
	"os"
	"path/filepath"
	"testing"

	"safeshell/internal/ai"
	"safeshell/internal/ai/providers"
	"safeshell/internal/models"
)

func write(t *testing.T, s string) string {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte(s), 0o600)
	return p
}

func TestDefaultsWhenMissing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if err != nil || c.AI.Mode != ai.Auto || c.AI.Local.Model != models.Default || c.AI.Local.GPU != "auto" {
		t.Errorf("%+v %v", c, err)
	}
}

func TestLoadsValuesAndEnvOverridesMode(t *testing.T) {
	p := write(t, `{"ai":{"mode":"hybrid","local":{"model":"qwen2.5-coder-7b","gpu":"off"},"cloud":{"provider":"gemini","model":"gemini-2.5-pro"}}}`)
	c, err := Load(p)
	if err != nil || c.AI.Mode != ai.Hybrid || c.AI.Local.Model != "qwen2.5-coder-7b" || c.AI.Cloud.Provider != "gemini" {
		t.Fatalf("%+v %v", c, err)
	}
	t.Setenv("SAFESHELL_AI", "off")
	if c, _ := LoadEffective(p); c.AI.Mode != ai.Off {
		t.Error("env override")
	}
	if c, _ := Load(p); c.AI.Mode != ai.Hybrid {
		t.Error("Load must return what is saved, not a temporary override")
	}
	t.Setenv("SAFESHELL_AI", "sometimes")
	if _, err := LoadEffective(p); err == nil {
		t.Error("a bad override must be reported")
	}
}

func TestRejectsBadValues(t *testing.T) {
	for name, s := range map[string]string{
		"mode":     `{"ai":{"mode":"always"}}`,
		"json":     `{`,
		"model":    `{"ai":{"local":{"model":"gpt-9000"}}}`,
		"provider": `{"ai":{"cloud":{"provider":"acme"}}}`,
		"gpu":      `{"ai":{"local":{"gpu":"metal"}}}`,
	} {
		if _, err := Load(write(t, s)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestSaveRoundTripsPrivately(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "config.json")
	var c Config
	c.AI.Mode = ai.Cloud
	c.AI.Cloud.Provider = "openai"
	c.AI.Local.Model = "qwen3-4b"
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode())
	}
	got, err := Load(p)
	if err != nil || got.AI.Mode != ai.Cloud || got.AI.Cloud.Provider != "openai" || got.AI.Local.Model != "qwen3-4b" {
		t.Errorf("%+v %v", got, err)
	}
}

func TestKeysEnvWinsAndFileIsPrivate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	p, _ := providers.Find("openai")
	a, _ := providers.Find("anthropic")
	t.Setenv("ANTHROPIC_API_KEY", "")
	if Key(p) != "" {
		t.Error("no key yet")
	}
	if err := SaveKey(p, "sk-file"); err != nil {
		t.Fatal(err)
	}
	if err := SaveKey(a, "sk-ant"); err != nil {
		t.Fatal(err)
	}
	if Key(p) != "sk-file" || Key(a) != "sk-ant" {
		t.Error("keys not kept apart")
	}
	if st, _ := os.Stat(KeysPath()); st.Mode().Perm() != 0o600 {
		t.Errorf("keys mode %v", st.Mode())
	}
	t.Setenv("OPENAI_API_KEY", "sk-env")
	if Key(p) != "sk-env" {
		t.Error("environment must win")
	}
}

func TestPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/c")
	if Path() != "/c/safeshell/config.json" {
		t.Error(Path())
	}
}
