package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendWritesOneJSONLinePerEntryPrivately(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state", "audit.jsonl")
	for _, l := range []string{"rm a", "rm b"} {
		if err := Append(p, Entry{Time: time.Unix(1, 0).UTC(), Line: l, Level: "R3", Decision: "block", Outcome: "cancelled"}); err != nil {
			t.Fatal(err)
		}
	}
	f, _ := os.Open(p)
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, e.Line)
	}
	if len(lines) != 2 || lines[0] != "rm a" || lines[1] != "rm b" {
		t.Errorf("lines %v", lines)
	}
	if st, _ := os.Stat(p); st.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", st.Mode())
	}
}

func TestDefaultPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/x")
	if got := DefaultPath(); got != "/x/safeshell/audit.jsonl" {
		t.Error(got)
	}
}

func TestAppendFailsCleanlyWhenUnwritable(t *testing.T) {
	if Append("/proc/nope/audit.jsonl", Entry{}) == nil {
		t.Error("expected an error")
	}
}
