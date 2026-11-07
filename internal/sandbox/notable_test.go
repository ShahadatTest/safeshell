package sandbox

import (
	"io/fs"
	"path/filepath"
	"testing"
)

func TestIsNotable(t *testing.T) {
	yes := []string{".git", "proj/.git/config", ".ssh/id_rsa", "a/id_ed25519", "x/.env", "thesis.PDF", "notes/report.docx", "data/app.sqlite3", "keys/server.pem", ".bashrc", "a/b/analysis.ipynb", "README.md"}
	no := []string{"", ".", "main.go", "build/out.o", "node_modules/x/index.js", "a.txt", "photo.jpg", "gitignore", "env"}
	for _, p := range yes {
		if !IsNotable(p) {
			t.Errorf("IsNotable(%q) = false", p)
		}
	}
	for _, p := range no {
		if IsNotable(p) {
			t.Errorf("IsNotable(%q) = true", p)
		}
	}
}

func TestDeletedTreeReportsNotableFiles(t *testing.T) {
	lower, upper := t.TempDir(), t.TempDir()
	write(t, lower, "proj/main.go", "x")
	write(t, lower, "proj/.git/HEAD", "ref")
	write(t, lower, "proj/docs/thesis.pdf", "pdf")
	for i := 0; i < 10; i++ {
		write(t, lower, "proj/notes/n"+string(rune('a'+i))+".md", "n")
	}
	write(t, upper, "proj", "")
	opt := Options{IsWhiteout: func(p string, _ fs.FileInfo) bool { return filepath.Base(p) == "proj" }}
	d, _ := Diff(upper, lower, opt)
	c := d.Changes[0]
	if len(c.Notable) != 5 {
		t.Fatalf("notable must be capped at 5, got %v", c.Notable)
	}
	found := false
	for _, n := range c.Notable {
		if filepath.Base(n) == ".git" || filepath.Base(n) == "thesis.pdf" {
			found = true
		}
		if filepath.Base(filepath.Dir(n)) == "" || n[:4] != "proj" {
			t.Errorf("notable paths should start at the deleted directory: %q", n)
		}
	}
	if !found {
		t.Errorf("expected .git or thesis.pdf among %v", c.Notable)
	}
}
