package report

import (
	"strings"
	"testing"

	"safeshell/internal/domain"
	"safeshell/internal/sandbox"
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
