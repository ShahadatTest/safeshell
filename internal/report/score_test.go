package report

import (
	"strings"
	"testing"

	"github.com/samarthsinh2660/safeshell/internal/domain"
	"github.com/samarthsinh2660/safeshell/internal/sandbox"
)

func diff(cs ...sandbox.Change) sandbox.FileDiff { return sandbox.FileDiff{Changes: cs} }

func TestScoreLevels(t *testing.T) {
	tests := []struct {
		name string
		d    sandbox.FileDiff
		want domain.Level
	}{
		{"nothing", diff(), domain.R0},
		{"creates a file", diff(sandbox.Change{Path: "home/me/a.txt", Kind: sandbox.Created, Files: 1}), domain.R1},
		{"edits a file", diff(sandbox.Change{Path: "home/me/a.txt", Kind: sandbox.Modified, Files: 1}), domain.R2},
		{"deletes one file", diff(sandbox.Change{Path: "tmp/x.log", Kind: sandbox.Deleted, Files: 1}), domain.R2},
		{"deletes many files", diff(sandbox.Change{Path: "home/me/cache", Kind: sandbox.Deleted, IsDir: true, Files: 4312}), domain.R3},
		{"just under the bulk threshold", diff(sandbox.Change{Path: "tmp/d", Kind: sandbox.Deleted, IsDir: true, Files: 99}), domain.R2},
		{"edits a system file", diff(sandbox.Change{Path: "etc/hosts", Kind: sandbox.Modified, Files: 1}), domain.R3},
		{"deletes under /usr", diff(sandbox.Change{Path: "usr/lib/x", Kind: sandbox.Deleted, Files: 1}), domain.R3},
		{"creates a file in /etc", diff(sandbox.Change{Path: "etc/new.conf", Kind: sandbox.Created, Files: 1}), domain.R1},
		{"deletes a git repo", diff(sandbox.Change{Path: "home/me/proj", Kind: sandbox.Deleted, IsDir: true, Files: 3, Notable: []string{"proj/.git"}}), domain.R3},
		{"deletes a document", diff(sandbox.Change{Path: "home/me/thesis.pdf", Kind: sandbox.Deleted, Files: 1}), domain.R3},
		{"deletes a ssh key", diff(sandbox.Change{Path: "home/me/.ssh/id_rsa", Kind: sandbox.Deleted, Files: 1}), domain.R3},
		{"a user dir called usr is not the system", diff(sandbox.Change{Path: "home/me/usr/x", Kind: sandbox.Deleted, Files: 1}), domain.R2},
	}
	for _, tt := range tests {
		got := Score(tt.d)
		if got.Level != tt.want {
			t.Errorf("%s: level %s, want %s (%v)", tt.name, got.Level, tt.want, got.Reasons)
		}
		if (got.Level > domain.R0) != (len(got.Reasons) > 0) {
			t.Errorf("%s: reasons %v do not match level %s", tt.name, got.Reasons, got.Level)
		}
	}
}

func TestScoreExplainsItself(t *testing.T) {
	im := Score(diff(
		sandbox.Change{Path: "etc/hosts", Kind: sandbox.Modified, Files: 1},
		sandbox.Change{Path: "home/me/p", Kind: sandbox.Deleted, IsDir: true, Files: 5, Notable: []string{"p/.git", "p/a.pdf", "p/b.pdf", "p/c.pdf"}},
	))
	joined := strings.Join(im.Reasons, "; ")
	for _, want := range []string{"changes system files: /etc/hosts", "deletes files that look important", "and 1 more"} {
		if !strings.Contains(joined, want) {
			t.Errorf("reasons missing %q: %s", want, joined)
		}
	}
}

func TestFormatHelpers(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 4312: "4,312", 1234567: "1,234,567"} {
		if got := Commas(n); got != want {
			t.Errorf("Commas(%d) = %q", n, got)
		}
	}
	if Files(1) != "1 file" || Files(2) != "2 files" || Files(0) != "0 files" || Files(4312) != "4,312 files" {
		t.Error("Files wrong")
	}
	for b, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KiB", 1536: "1.5 KiB", 2 << 30: "2.0 GiB"} {
		if got := HumanBytes(b); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", b, got, want)
		}
	}
}

func TestFileEffect(t *testing.T) {
	e := FileEffect(diff(
		sandbox.Change{Path: "home/me/p", Kind: sandbox.Deleted, IsDir: true, Files: 4312, Bytes: 2 << 30, Notable: []string{"p/.git"}},
		sandbox.Change{Path: "tmp/n", Kind: sandbox.Created, Files: 1},
	))
	if e.Evidence != domain.Observed || e.Kind != domain.EffectFS || e.MinLevel != domain.R3 {
		t.Errorf("%+v", e)
	}
	if !strings.Contains(e.Summary, "delete 4,312 files (2.0 GiB)") || !strings.Contains(e.Summary, "create 1 file") {
		t.Errorf("summary: %s", e.Summary)
	}
	joined := strings.Join(e.Details, "\n")
	for _, want := range []string{"because it deletes 4312 files", "deleted /home/me/p/ (4,312 files)", "created /tmp/n"} {
		if !strings.Contains(joined, want) {
			t.Errorf("details missing %q:\n%s", want, joined)
		}
	}
}

func TestFileEffectEmptyAndTruncatedAndLong(t *testing.T) {
	if e := FileEffect(diff()); e.Summary != "changes no files" || e.MinLevel != domain.R0 {
		t.Errorf("%+v", e)
	}
	if e := FileEffect(sandbox.FileDiff{Truncated: true, Changes: []sandbox.Change{{Path: "x", Kind: sandbox.Deleted, Files: 500000, IsDir: true}}}); !strings.Contains(e.Summary, "counting stopped") {
		t.Errorf("%s", e.Summary)
	}
	var cs []sandbox.Change
	for i := 0; i < 12; i++ {
		cs = append(cs, sandbox.Change{Path: "f", Kind: sandbox.Created, Files: 1})
	}
	e := FileEffect(diff(cs...))
	if last := e.Details[len(e.Details)-1]; last != "... and 4 more changes" {
		t.Errorf("long lists are cut: %q", last)
	}
}
