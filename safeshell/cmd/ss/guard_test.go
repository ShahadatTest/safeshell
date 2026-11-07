package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"safeshell/internal/ai"
	"safeshell/internal/audit"
	"strings"
	"testing"

	"safeshell/internal/domain"
)

func answer(t *testing.T, s string) {
	t.Helper()
	old := guardInput
	t.Cleanup(func() { guardInput = old })
	guardInput = func() io.Reader { return strings.NewReader(s) }
}

func TestGuardAllowedCommandPrintsItWithoutAsking(t *testing.T) {
	a := sampleAnalysis()
	a.Level, a.Decision = domain.R0, domain.Allow
	fakeAnalyze(t, a)
	answer(t, "")
	code, out, errOut := runSS("guard", "--cwd", "/work", "--", "ls", "-l")
	if code != 0 || out != "ls -l\n" || errOut != "" {
		t.Errorf("code %d out %q err %q", code, out, errOut)
	}
}

func TestGuardPassesCwdAndLine(t *testing.T) {
	got := fakeAnalyze(t, sampleAnalysis())
	answer(t, "c\n")
	runSS("guard", "--cwd", "/work", "--", "rm -rf x")
	if got[0] != "rm -rf x" || got[1] != "/work" {
		t.Errorf("analyze got %v", *got)
	}
}

func TestGuardBlockedCancelledByDefault(t *testing.T) {
	fakeAnalyze(t, sampleAnalysis())
	answer(t, "\n")
	code, out, errOut := runSS("guard", "--", "rm -rf ~/projects")
	if code != 1 || out != "" || !strings.Contains(errOut, "Nothing was run") || !strings.Contains(errOut, "BLOCK") {
		t.Errorf("code %d out %q err %q", code, out, errOut)
	}
}

func TestGuardSaferOptionReplacesTheCommand(t *testing.T) {
	fakeAnalyze(t, sampleAnalysis())
	answer(t, "1\n")
	code, out, _ := runSS("guard", "--", "rm -rf ~/projects")
	if code != 0 || out != "trash-put ~/projects\n" {
		t.Errorf("code %d out %q", code, out)
	}
}

func TestGuardDoItAnywayNeedsConfirmation(t *testing.T) {
	fakeAnalyze(t, sampleAnalysis())
	answer(t, "d\nyes\n")
	if code, out, _ := runSS("guard", "--", "rm -rf ~/projects"); code != 0 || out != "rm -rf ~/projects\n" {
		t.Errorf("code %d out %q", code, out)
	}
	answer(t, "d\nnope\n")
	if code, out, _ := runSS("guard", "--", "rm -rf ~/projects"); code != 1 || out != "" {
		t.Errorf("code %d out %q", code, out)
	}
}

func TestGuardUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"guard"}, {"guard", "--"}, {"guard", "--bogus", "x"}, {"hook"}, {"hook", "fish"}} {
		if code, _, _ := runSS(args...); code != 2 {
			t.Errorf("%v: code %d, want 2", args, code)
		}
	}
}

func TestHookPrintsScript(t *testing.T) {
	code, out, _ := runSS("hook", "bash")
	if code != 0 || !strings.Contains(out, "bind -x") {
		t.Errorf("code %d out %q", code, out)
	}
	if _, out, _ := runSS("hook", "zsh"); !strings.Contains(out, "zle -N accept-line") {
		t.Errorf("zsh: %q", out)
	}
}

func TestMain(m *testing.M) { // never write the real audit log from tests
	d, _ := os.MkdirTemp("", "ss-audit")
	auditPath = func() string { return filepath.Join(d, "audit.jsonl") }
	backend = func() (ai.Backend, error) { return nil, nil } // tests never reach a real model
	code := m.Run()
	os.RemoveAll(d)
	os.Exit(code)
}

func lastAudit(t *testing.T) audit.Entry {
	t.Helper()
	b, err := os.ReadFile(auditPath())
	if err != nil {
		t.Fatal(err)
	}
	ls := strings.Split(strings.TrimSpace(string(b)), "\n")
	var e audit.Entry
	if err := json.Unmarshal([]byte(ls[len(ls)-1]), &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestGuardLogsEveryOutcome(t *testing.T) {
	for _, tc := range []struct{ in, outcome, run string }{
		{"c\n", "cancelled", ""},
		{"1\n", "replaced", "trash-put ~/projects"},
		{"d\nyes\n", "ran", ""},
	} {
		fakeAnalyze(t, sampleAnalysis())
		answer(t, tc.in)
		runSS("guard", "--", "rm -rf ~/projects")
		e := lastAudit(t)
		if e.Outcome != tc.outcome || e.Run != tc.run || e.Level != "R3" || e.Decision != "block" || e.Line != "rm -rf ~/projects" {
			t.Errorf("%q: %+v", tc.in, e)
		}
	}
	a := sampleAnalysis()
	a.Level, a.Decision = domain.R0, domain.Allow
	fakeAnalyze(t, a)
	runSS("guard", "--", "ls")
	if e := lastAudit(t); e.Outcome != "allowed" {
		t.Errorf("%+v", e)
	}
}
