package main

import (
	"io"
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
