package effects

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"safeshell/internal/domain"
	"safeshell/internal/shellparse"
)

// fakeProc writes one process into a fake /proc directory.
func fakeProc(t *testing.T, root string, pid, ppid int, comm, cmdline string) {
	t.Helper()
	d := filepath.Join(root, fmt.Sprint(pid))
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	stat := fmt.Sprintf("%d (%s) S %d 1 1 0 -1 4194560 0 0 0 0 0 0 0 0 20 0 1 0 100 0 0", pid, comm, ppid)
	os.WriteFile(filepath.Join(d, "stat"), []byte(stat), 0o644)
	os.WriteFile(filepath.Join(d, "cmdline"), []byte(strings.ReplaceAll(cmdline, " ", "\x00")), 0o644)
}

func cmd(name string, args ...string) shellparse.Command {
	return shellparse.Command{Name: name, Args: args}
}

func world(t *testing.T) (ProcFS, int) {
	root := t.TempDir()
	me := os.Getuid()
	fakeProc(t, root, 1, 0, "systemd", "/sbin/init")
	fakeProc(t, root, 100, 1, "bash", "-bash")         // the user's shell
	fakeProc(t, root, 200, 100, "ss", "ss hook-check") // the caller
	fakeProc(t, root, 300, 1, "firefox", "/usr/lib/firefox/firefox --new-window")
	fakeProc(t, root, 301, 300, "firefox", "/usr/lib/firefox/firefox -contentproc")
	fakeProc(t, root, 400, 1, "node", "node server.js --port 3000")
	fakeProc(t, root, 500, 1, "sshd", "sshd: /usr/sbin/sshd -D")
	fakeProc(t, root, 600, 1, "my (odd) name", "weird")
	return ProcFS{Root: root}, me
}

func TestParseStatHandlesOddNames(t *testing.T) {
	comm, ppid, ok := parseStat("600 (my (odd) name) S 1 1 1 0")
	if !ok || comm != "my (odd) name" || ppid != 1 {
		t.Errorf("got %q %d %v", comm, ppid, ok)
	}
	for _, bad := range []string{"", "12 no parens S 1", "12 (x)", "12 (x) S notanumber"} {
		if _, _, ok := parseStat(bad); ok {
			t.Errorf("parseStat(%q) should fail", bad)
		}
	}
}

func TestListSkipsKernelThreadsAndJunk(t *testing.T) {
	fs, _ := world(t)
	os.MkdirAll(filepath.Join(fs.Root, "999"), 0o755) // no stat file
	os.MkdirAll(filepath.Join(fs.Root, "self"), 0o755)
	fakeProc(t, fs.Root, 700, 2, "kworker", "") // empty cmdline = kernel thread
	got := fs.List()
	if len(got) != 8 {
		t.Errorf("got %d processes: %+v", len(got), got)
	}
}

func TestAncestors(t *testing.T) {
	fs, _ := world(t)
	if got := fs.Ancestors(200); len(got) != 2 || got[0] != 100 || got[1] != 1 {
		t.Errorf("ancestors = %v", got)
	}
	if got := fs.Ancestors(4242); len(got) != 0 {
		t.Errorf("unknown pid has no ancestors: %v", got)
	}
}

func TestPkillByNameFindsAllMatches(t *testing.T) {
	fs, me := world(t)
	fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("pkill", "firefox")})
	if len(fx) != 1 || fx[0].Evidence != domain.Inspected || fx[0].Kind != domain.EffectProcess {
		t.Fatalf("%+v", fx)
	}
	if !strings.Contains(fx[0].Summary, "2 running") || len(fx[0].Details) != 2 || fx[0].MinLevel != domain.R2 {
		t.Errorf("%+v", fx[0])
	}
	if !strings.Contains(fx[0].Details[0], "yours") {
		t.Errorf("should say who owns it: %v", fx[0].Details)
	}
}

func TestPkillFullMatchAndExact(t *testing.T) {
	fs, me := world(t)
	// "3000" is only in a command line, not a process name
	if fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("pkill", "3000")}); !strings.Contains(fx[0].Summary, "no running process") {
		t.Errorf("without -f only the name is matched: %+v", fx)
	}
	if fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("pkill", "-f", "3000")}); !strings.Contains(fx[0].Summary, "1 running") {
		t.Errorf("-f matches the full command line: %+v", fx)
	}
	// killall is an exact name match: "fire" must not hit firefox
	if fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("killall", "fire")}); !strings.Contains(fx[0].Summary, "no running process") {
		t.Errorf("killall is exact: %+v", fx)
	}
	if fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("killall", "node")}); !strings.Contains(fx[0].Summary, "1 running") {
		t.Errorf("%+v", fx)
	}
}

func TestKillByPid(t *testing.T) {
	fs, me := world(t)
	fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("kill", "-9", "400", "31337")})
	if len(fx) != 1 || !strings.Contains(fx[0].Summary, "1 running") || !strings.Contains(fx[0].Details[0], "node") {
		t.Errorf("%+v", fx)
	}
	if fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("kill", "31337")}); len(fx) != 0 {
		t.Errorf("a pid that does not exist has no effect: %+v", fx)
	}
}

func TestCriticalTargetsRaiseToR4(t *testing.T) {
	fs, me := world(t)
	for name, c := range map[string]shellparse.Command{
		"init":          cmd("kill", "1"),
		"sshd":          cmd("pkill", "sshd"),
		"your shell":    cmd("kill", "100"),
		"the caller":    cmd("pkill", "-f", "hook-check"), // self is excluded, so nothing matches
		"the shell too": cmd("pkill", "bash"),
	} {
		fx := InspectProcesses(fs, 200, me, []shellparse.Command{c})
		if name == "the caller" {
			if fx[0].MinLevel != domain.R0 {
				t.Errorf("%s: the calling process is never a target: %+v", name, fx)
			}
			continue
		}
		if len(fx) != 1 || fx[0].MinLevel != domain.R4 {
			t.Errorf("%s: want R4, got %+v", name, fx)
		}
	}
}

func TestBulkKillIsR3(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 25; i++ {
		fakeProc(t, root, 1000+i, 1, "worker", "worker --n")
	}
	fakeProc(t, root, 1, 0, "systemd", "init")
	fx := InspectProcesses(ProcFS{Root: root}, 1, os.Getuid(), []shellparse.Command{cmd("pkill", "worker")})
	if fx[0].MinLevel != domain.R3 {
		t.Errorf("%+v", fx[0])
	}
}

func TestBadPatternIsEstimated(t *testing.T) {
	fs, me := world(t)
	fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("pkill", "([")})
	if len(fx) != 1 || fx[0].Evidence != domain.Estimated || fx[0].MinLevel != domain.R2 {
		t.Errorf("%+v", fx)
	}
}

func TestUserFilterIsFlagged(t *testing.T) {
	fs, me := world(t)
	fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("pkill", "-u", "bob", "node")})
	if len(fx) != 1 || !strings.Contains(fx[0].Summary, "-u user filter was not applied") {
		t.Errorf("%+v", fx)
	}
}

func TestOtherCommandsAreIgnored(t *testing.T) {
	fs, me := world(t)
	if fx := InspectProcesses(fs, 200, me, []shellparse.Command{cmd("ls"), cmd("rm", "x")}); len(fx) != 0 {
		t.Errorf("%+v", fx)
	}
}

func TestOwnerAndClip(t *testing.T) {
	if owner(1000, 1000) != "yours" || owner(0, 1000) != "root" || owner(33, 1000) != "uid 33" || owner(-1, 1000) != "unknown owner" {
		t.Error("owner labels")
	}
	if clip("abc", 5) != "abc" || clip("abcdefgh", 5) != "abcd…" {
		t.Error("clip")
	}
}

func TestRealProcFS(t *testing.T) {
	fs := ProcFS{Root: "/proc"}
	if _, err := os.Stat("/proc/self"); err != nil {
		t.Skip("no /proc")
	}
	self := fs.List()
	found := false
	for _, p := range self {
		if p.PID == os.Getpid() {
			found = true
			if p.UID != os.Getuid() {
				t.Errorf("uid %d != %d", p.UID, os.Getuid())
			}
		}
	}
	if !found {
		t.Error("this test process is not in the list")
	}
}
