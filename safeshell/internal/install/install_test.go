package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rc(t *testing.T, content string) Shell {
	p := filepath.Join(t.TempDir(), ".bashrc")
	if content != "\x00" {
		os.WriteFile(p, []byte(content), 0o600)
	}
	return Shell{"bash", p}
}

func read(t *testing.T, sh Shell) string {
	b, _ := os.ReadFile(sh.RC)
	return string(b)
}

func TestDetect(t *testing.T) {
	if s, err := Detect("/usr/bin/zsh", "/h"); err != nil || s.Name != "zsh" || s.RC != "/h/.zshrc" {
		t.Errorf("%+v %v", s, err)
	}
	if s, _ := Detect("/bin/bash", "/h"); s.RC != "/h/.bashrc" {
		t.Error(s)
	}
	if _, err := Detect("/usr/bin/fish", "/h"); err == nil || !strings.Contains(err.Error(), "fish") {
		t.Errorf("%v", err)
	}
}

func TestAddKeepsTheExistingFileBacksItUpAndIsIdempotent(t *testing.T) {
	orig := "export A=1\nalias ll='ls -l'"
	sh := rc(t, orig)
	changed, err := Add(sh, "/home/me/.local/bin/ss")
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	got := read(t, sh)
	if !strings.HasPrefix(got, orig+"\n") || !strings.Contains(got, `hook bash)"`) || !strings.Contains(got, "'/home/me/.local/bin/ss'") {
		t.Errorf("%q", got)
	}
	if b, _ := os.ReadFile(sh.RC + ".safeshell-backup"); string(b) != orig {
		t.Error("no faithful backup")
	}
	if changed, _ := Add(sh, "/x/ss"); changed || read(t, sh) != got {
		t.Error("adding twice must change nothing")
	}
	if !Installed(sh) {
		t.Error("Installed")
	}
}

func TestAddCreatesAMissingFileAndQuotesOddPaths(t *testing.T) {
	sh := rc(t, "\x00")
	if _, err := Add(sh, "/home/o'brien/ss"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, sh), `'/home/o'\''brien/ss'`) {
		t.Errorf("%q", read(t, sh))
	}
}

func TestRemoveRestoresTheFileExactly(t *testing.T) {
	orig := "export A=1\n\nalias x=y\n"
	sh := rc(t, orig)
	Add(sh, "/ss")
	if changed, err := Remove(sh); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if got := read(t, sh); got != "export A=1\n\nalias x=y\n" {
		t.Errorf("%q", got)
	}
	if changed, _ := Remove(sh); changed {
		t.Error("removing twice")
	}
	if Installed(sh) {
		t.Error("still installed")
	}
	if changed, err := Remove(rc(t, "\x00")); changed || err != nil {
		t.Error("missing file")
	}
}

func TestRemoveKeepsLinesAfterTheBlock(t *testing.T) {
	sh := rc(t, "")
	Add(sh, "/ss")
	os.WriteFile(sh.RC, []byte(read(t, sh)+"export AFTER=1\n"), 0o644)
	Remove(sh)
	if got := read(t, sh); got != "export AFTER=1\n" {
		t.Errorf("%q", got)
	}
}
