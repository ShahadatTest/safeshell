package shellhook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptsExistAndParse(t *testing.T) {
	for _, sh := range []string{"bash", "zsh"} {
		s, err := Script(sh)
		if err != nil || !strings.Contains(s, "safeshell guard") {
			t.Fatalf("%s: %v", sh, err)
		}
		bin, err := exec.LookPath(sh)
		if err != nil {
			continue
		}
		f := filepath.Join(t.TempDir(), "hook")
		_ = os.WriteFile(f, []byte(s), 0o600)
		if out, err := exec.Command(bin, "-n", f).CombinedOutput(); err != nil {
			t.Errorf("%s syntax: %v %s", sh, err, out)
		}
	}
	if _, err := Script("fish"); err == nil {
		t.Error("fish is not supported yet and must say so")
	}
}
