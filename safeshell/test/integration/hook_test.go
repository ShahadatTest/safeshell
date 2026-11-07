package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// term is a bash session on a pseudo-terminal, as a person would use it.
type term struct {
	master *os.File
	mu     sync.Mutex
	buf    strings.Builder
}

func startBash(t *testing.T, extraEnv ...string) *term {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skip("no pty:", err)
	}
	n, _ := unix.IoctlGetInt(int(m.Fd()), unix.TIOCGPTN)
	_ = unix.IoctlSetPointerInt(int(m.Fd()), unix.TIOCSPTLCK, 0)
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skip("no pty:", err)
	}
	c := exec.Command("bash", "--norc", "--noprofile", "-i")
	c.Env = append(append(os.Environ(), extraEnv...), "PS1=READY> ", "XDG_STATE_HOME="+t.TempDir(), "XDG_DATA_HOME="+t.TempDir(), "PATH="+filepath.Dir(ssBin)+":"+os.Getenv("PATH"))
	c.Stdin, c.Stdout, c.Stderr = slave, slave, slave
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	slave.Close()
	tm := &term{master: m}
	go func() {
		b := make([]byte, 4096)
		for {
			n, err := m.Read(b)
			if n > 0 {
				tm.mu.Lock()
				tm.buf.Write(b[:n])
				tm.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { _ = c.Process.Kill(); _ = c.Wait(); m.Close() })
	return tm
}

// send types s, after forgetting what was on screen so waitFor sees only new output.
func (tm *term) send(s string) {
	tm.mu.Lock()
	tm.buf.Reset()
	tm.mu.Unlock()
	_, _ = tm.master.WriteString(s)
}

func (tm *term) waitFor(t *testing.T, want string) {
	t.Helper()
	for end := time.Now().Add(15 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		tm.mu.Lock()
		ok := strings.Contains(tm.buf.String(), want)
		tm.mu.Unlock()
		if ok {
			return
		}
	}
	tm.mu.Lock()
	defer tm.mu.Unlock()
	t.Fatalf("never saw %q; terminal showed:\n%s", want, tm.buf.String())
}

func TestShellHookStopsADangerousCommandAndLetsSafeOnesRun(t *testing.T) {
	needSandbox(t)
	d := t.TempDir()
	victim := filepath.Join(d, "proj")
	_ = os.MkdirAll(victim, 0o755)
	_ = os.WriteFile(filepath.Join(victim, "a.txt"), []byte("x"), 0o644)

	sh := startBash(t)
	sh.waitFor(t, "READY>")
	sh.send(`eval "$(ss hook bash)"` + "\r")
	sh.send("rm -rf " + victim + "\r")
	sh.waitFor(t, "[c] cancel")
	sh.send("c\r")
	sh.waitFor(t, "Nothing was run")
	if _, err := os.Stat(filepath.Join(victim, "a.txt")); err != nil {
		t.Fatal("the dangerous command ran")
	}

	sh.send("echo from-the-hook\r")
	sh.waitFor(t, "from-the-hook\r\n")

	// "Do it anyway" really runs it.
	sh.send("rm -rf " + victim + "\r")
	sh.waitFor(t, "[c] cancel")
	sh.send("d\r")
	sh.waitFor(t, "Type yes")
	sh.send("yes\r")
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if _, err := os.Stat(victim); err != nil {
			break
		}
	}
	if _, err := os.Stat(victim); err == nil {
		t.Fatal("do-it-anyway did not run the command")
	}

	// ...and undo brings it back, verified.
	sh.send("ss undo\r")
	sh.waitFor(t, "Verified")
	if b, err := os.ReadFile(filepath.Join(victim, "a.txt")); err != nil || string(b) != "x" {
		t.Errorf("undo did not restore the file: %v %q", err, b)
	}
}

// fakeModel serves Ollama's API and answers every chat with reply.
func fakeModel(t *testing.T, reply string) (cfgDir string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_, _ = w.Write([]byte(`{"models":[{"name":"test-model"}]}`))
			return
		}
		b, _ := json.Marshal(map[string]any{"message": map[string]string{"content": reply}})
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)
	cfgDir = t.TempDir()
	_ = os.MkdirAll(filepath.Join(cfgDir, "safeshell"), 0o755)
	cfg := `{"ai":{"mode":"local","local":{"url":"` + srv.URL + `","model":"test-model"}}}`
	_ = os.WriteFile(filepath.Join(cfgDir, "safeshell", "config.json"), []byte(cfg), 0o600)
	return cfgDir
}

func TestTypingEnglishInTheTerminalRunsTheTranslatedCommand(t *testing.T) {
	needSandbox(t)
	cfg := fakeModel(t, `{"command":"echo translated-by-model","note":"prints a word"}`)
	sh := startBash(t, "XDG_CONFIG_HOME="+cfg)
	sh.waitFor(t, "READY>")
	sh.send(`eval "$(ss hook bash)"` + "\r")
	sh.send("please say something nice\r")
	sh.waitFor(t, "Proposed: echo translated-by-model")
	sh.waitFor(t, "Run it? [Y/n]")
	sh.send("\r")
	sh.waitFor(t, "translated-by-model\r\n")

	// The same works with the explicit ? prefix, and a real command is untouched.
	sh.send("? anything at all\r")
	sh.waitFor(t, "Run it? [Y/n]")
	sh.send("n\r")
	sh.waitFor(t, "Nothing was run")
	sh.send("echo plain-command\r")
	sh.waitFor(t, "plain-command\r\n")
}

func TestADangerousTranslationIsStillStopped(t *testing.T) {
	needSandbox(t)
	d := t.TempDir()
	victim := filepath.Join(d, "proj")
	_ = os.MkdirAll(victim, 0o755)
	_ = os.WriteFile(filepath.Join(victim, "a.txt"), []byte("x"), 0o644)
	cfg := fakeModel(t, `{"command":"rm -rf `+victim+`","note":"removes it"}`)
	sh := startBash(t, "XDG_CONFIG_HOME="+cfg)
	sh.waitFor(t, "READY>")
	sh.send(`eval "$(ss hook bash)"` + "\r")
	sh.send("get rid of the project folder\r")
	sh.waitFor(t, "[c] cancel")
	sh.send("c\r")
	sh.waitFor(t, "Nothing was run")
	if _, err := os.Stat(filepath.Join(victim, "a.txt")); err != nil {
		t.Fatal("the model's command ran without passing the safety check")
	}
}
