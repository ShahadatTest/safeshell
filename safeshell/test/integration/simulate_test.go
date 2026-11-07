// Package integration runs the real ss binary against the real kernel. It needs
// bin/ss (make build) and a machine where `ss doctor` reports the sandbox as
// available; otherwise every test is skipped.
package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type change struct {
	Path  string
	Kind  string
	IsDir bool
	Files int
	Bytes int64
}

type result struct {
	ExitCode  int
	TimedOut  bool
	Stdout    string
	Stderr    string
	Diff      struct{ Changes []change }
	Truncated bool
	Warnings  []string
}

var ssBin string

func TestMain(m *testing.M) {
	ssBin, _ = filepath.Abs("../../bin/ss")
	os.Exit(m.Run())
}

func needSandbox(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(ssBin); err != nil {
		t.Skip("bin/ss not built (run: make build)")
	}
	if err := exec.Command(ssBin, "doctor").Run(); err != nil {
		t.Skip("sandbox not available here (run: ss doctor)")
	}
}

func simulate(t *testing.T, args ...string) result {
	t.Helper()
	out, err := exec.Command(ssBin, append([]string{"simulate", "--json"}, args...)...).Output()
	if err != nil {
		t.Fatalf("ss simulate %v: %v\n%s", args, err, out)
	}
	var r result
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	return r
}

func (r result) find(rel string) (change, bool) {
	for _, c := range r.Diff.Changes {
		if strings.HasSuffix(c.Path, rel) {
			return c, true
		}
	}
	return change{}, false
}

func fixture(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(d, "proj", "sub"), 0o755))
	must(os.WriteFile(filepath.Join(d, "proj", "a.txt"), []byte("aaaa"), 0o644))
	must(os.WriteFile(filepath.Join(d, "proj", "sub", "b.txt"), []byte("bb"), 0o644))
	must(os.WriteFile(filepath.Join(d, "keep.txt"), []byte("keep"), 0o644))
	return d
}

func TestDeleteIsObservedAndRealFilesSurvive(t *testing.T) {
	needSandbox(t)
	d := fixture(t)
	r := simulate(t, "--cwd", d, "rm -rf proj")
	c, ok := r.find("/proj")
	if !ok || c.Kind != "deleted" || !c.IsDir || c.Files != 2 || c.Bytes != 6 {
		t.Fatalf("deleted dir not observed correctly: %+v (all: %+v)", c, r.Diff.Changes)
	}
	for _, f := range []string{"proj/a.txt", "proj/sub/b.txt", "keep.txt"} {
		if _, err := os.Stat(filepath.Join(d, f)); err != nil {
			t.Errorf("real file %s was touched by the simulation: %v", f, err)
		}
	}
}

func TestCreateAndModifyAreObserved(t *testing.T) {
	needSandbox(t)
	d := fixture(t)
	r := simulate(t, "--cwd", d, "echo hi > new.txt; echo more >> keep.txt")
	if c, ok := r.find("/new.txt"); !ok || c.Kind != "created" {
		t.Errorf("new.txt: %+v", c)
	}
	if c, ok := r.find("/keep.txt"); !ok || c.Kind != "modified" {
		t.Errorf("keep.txt: %+v", c)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "keep.txt")); string(b) != "keep" {
		t.Errorf("real keep.txt changed to %q", b)
	}
	if _, err := os.Stat(filepath.Join(d, "new.txt")); err == nil {
		t.Error("new.txt exists on the real filesystem")
	}
}

func TestWriteToHomeNeverReachesTheRealHome(t *testing.T) {
	needSandbox(t)
	home, _ := os.UserHomeDir()
	name := ".ss-sim-canary-" + time.Now().Format("150405.000")
	r := simulate(t, "echo x > ~/"+name)
	if _, ok := r.find(name); !ok {
		t.Errorf("canary not in the diff: %+v", r.Diff.Changes)
	}
	if _, err := os.Stat(filepath.Join(home, name)); err == nil {
		os.Remove(filepath.Join(home, name))
		t.Fatal("the simulation wrote into the real home directory")
	}
}

func TestExitCodeStdoutAndStderrAreReported(t *testing.T) {
	needSandbox(t)
	r := simulate(t, "echo out; echo err >&2; exit 7")
	if r.ExitCode != 7 || strings.TrimSpace(r.Stdout) != "out" || strings.TrimSpace(r.Stderr) != "err" {
		t.Errorf("%+v", r)
	}
}

func TestTimeoutKillsTheWholeTree(t *testing.T) {
	needSandbox(t)
	start := time.Now()
	r := simulate(t, "--timeout", "1s", "sleep 30 & sleep 30; echo unreachable")
	if !r.TimedOut || r.ExitCode != -1 || strings.Contains(r.Stdout, "unreachable") {
		t.Errorf("%+v", r)
	}
	if time.Since(start) > 10*time.Second {
		t.Errorf("took %s: background processes were not killed", time.Since(start))
	}
}

func TestNetworkIsUnreachable(t *testing.T) {
	needSandbox(t)
	r := simulate(t, `python3 -c "import socket; socket.create_connection(('1.1.1.1', 80), 2)"`)
	if r.ExitCode == 0 {
		t.Error("a connection to the internet succeeded inside the sandbox")
	}
}

func TestSandboxHasItsOwnProcessAndHostname(t *testing.T) {
	needSandbox(t)
	r := simulate(t, "echo $$; hostname")
	lines := strings.Fields(r.Stdout)
	if len(lines) < 2 || lines[1] != "safeshell" {
		t.Fatalf("stdout = %q", r.Stdout)
	}
	if lines[0] == "" || len(lines[0]) > 3 {
		t.Errorf("PID %q does not look like a fresh PID namespace", lines[0])
	}
}

func TestOutputIsCapped(t *testing.T) {
	needSandbox(t)
	r := simulate(t, "head -c 500000 /dev/zero | tr '\\0' x")
	if !r.Truncated || len(r.Stdout) > 70<<10 {
		t.Errorf("truncated=%v len=%d", r.Truncated, len(r.Stdout))
	}
}

func TestNoScratchDirectoriesLeakAndRunsDoNotInterfere(t *testing.T) {
	needSandbox(t)
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	before, _ := filepath.Glob(filepath.Join(base, "ss-sim-*"))

	var wg sync.WaitGroup
	errs := make(chan string, 8)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			d := fixture(t)
			r := simulate(t, "--cwd", d, "rm -rf proj; echo n > only-"+string(rune('a'+i)))
			if _, ok := r.find("/proj"); !ok {
				errs <- "missing proj delete"
			}
			for j := 0; j < 4; j++ {
				if j != i {
					if _, ok := r.find("/only-" + string(rune('a'+j))); ok {
						errs <- "saw another run's file"
					}
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	after, _ := filepath.Glob(filepath.Join(base, "ss-sim-*"))
	if len(after) > len(before) {
		t.Errorf("scratch directories leaked: %v", after)
	}
}

func TestRelativePathsUseTheGivenDirectory(t *testing.T) {
	needSandbox(t)
	d := fixture(t)
	r := simulate(t, "--cwd", d, "pwd; cat keep.txt")
	if !strings.Contains(r.Stdout, d) || !strings.Contains(r.Stdout, "keep") {
		t.Errorf("stdout = %q", r.Stdout)
	}
}
