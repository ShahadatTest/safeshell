package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// IdleSeconds is how long the server keeps the model in memory without a request.
const IdleSeconds = 600

// Spec says what to run.
type Spec struct {
	Bin   string // llama-server
	Model string // path to the .gguf
	GPU   bool
}

type state struct {
	PID   int    `json:"pid"`
	Port  int    `json:"port"`
	Model string `json:"model"`
	Key   string `json:"key"` // the server only answers requests that carry it
}

// Endpoint is how to reach a running server.
type Endpoint struct {
	Port int
	Key  string
}

func (d Dirs) statePath() string { return filepath.Join(d.State, "llama.json") }

func (d Dirs) readState() (state, bool) {
	var s state
	b, err := os.ReadFile(d.statePath())
	if err != nil || json.Unmarshal(b, &s) != nil || s.PID <= 0 {
		return state{}, false
	}
	return s, true
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func healthy(ctx context.Context, port int) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Ensure returns the endpoint of a running server for spec, starting one if needed. Starts are serialised with a lock
// so two shells opening at once do not launch two servers.
func (d Dirs) Ensure(ctx context.Context, spec Spec, log io.Writer) (Endpoint, error) {
	if err := os.MkdirAll(d.State, 0o700); err != nil {
		return Endpoint{}, err
	}
	lock, err := os.OpenFile(filepath.Join(d.State, "llama.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return Endpoint{}, err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return Endpoint{}, err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	if s, ok := d.readState(); ok && alive(s.PID) {
		if s.Model == spec.Model && healthy(ctx, s.Port) {
			return Endpoint{s.Port, s.Key}, nil
		}
		_ = syscall.Kill(s.PID, syscall.SIGTERM) // a different model was asked for, or it is wedged
		time.Sleep(300 * time.Millisecond)
	}
	if log != nil {
		fmt.Fprintln(log, "Starting the local AI model...")
	}
	port, err := freePort()
	if err != nil {
		return Endpoint{}, err
	}
	key, err := randomKey()
	if err != nil {
		return Endpoint{}, err
	}
	logf, _ := os.OpenFile(filepath.Join(d.State, "llama.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	args := []string{"-m", spec.Model, "--host", "127.0.0.1", "--port", strconv.Itoa(port),
		"--api-key", key, "-c", "4096", "--parallel", "1", "--no-webui", "--sleep-idle-seconds", strconv.Itoa(IdleSeconds)}
	if !spec.GPU {
		args = append(args, "-ngl", "0")
	}
	cmd := exec.Command(spec.Bin, args...)
	cmd.Dir = filepath.Dir(spec.Bin)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(spec.Bin))
	cmd.Stdout, cmd.Stderr = logf, logf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // outlives the shell command that started it
	if err := cmd.Start(); err != nil {
		return Endpoint{}, fmt.Errorf("could not start the local engine: %w", err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	b, _ := json.Marshal(state{PID: cmd.Process.Pid, Port: port, Model: spec.Model, Key: key})
	_ = os.WriteFile(d.statePath(), b, 0o600)

	for end := time.Now().Add(180 * time.Second); time.Now().Before(end); {
		select {
		case <-exited:
			return Endpoint{}, fmt.Errorf("the local engine stopped while starting (see %s)", filepath.Join(d.State, "llama.log"))
		case <-ctx.Done():
			return Endpoint{}, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
		if healthy(ctx, port) {
			return Endpoint{port, key}, nil
		}
	}
	return Endpoint{}, errors.New("the local engine did not become ready within 3 minutes")
}

// Stop ends the background server, if one is running.
func (d Dirs) Stop() bool {
	s, ok := d.readState()
	if !ok || !alive(s.PID) {
		return false
	}
	_ = syscall.Kill(s.PID, syscall.SIGTERM)
	_ = os.Remove(d.statePath())
	return true
}

// Running reports the background server's state without starting it.
func (d Dirs) Running() (model string, ok bool) {
	s, found := d.readState()
	if !found || !alive(s.PID) {
		return "", false
	}
	return s.Model, true
}

func randomKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
