package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func start(t *testing.T, s *Server) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s.sock")
	l, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Serve(ctx, l); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return path
}

func echo() map[string]Handler {
	return map[string]Handler{
		"ping": func(context.Context, json.RawMessage) (any, error) { return map[string]int{"protocol": Protocol}, nil },
		"add": func(_ context.Context, p json.RawMessage) (any, error) {
			var a struct{ A, B int }
			if err := json.Unmarshal(p, &a); err != nil {
				return nil, &Error{CodeBadParams, "need A and B"}
			}
			return a.A + a.B, nil
		},
		"boom": func(context.Context, json.RawMessage) (any, error) { panic("handler bug") },
		"fail": func(context.Context, json.RawMessage) (any, error) { return nil, errors.New("disk on fire") },
		"slow": func(ctx context.Context, _ json.RawMessage) (any, error) { <-ctx.Done(); return nil, ctx.Err() },
	}
}

func TestCallRoundTripAndErrors(t *testing.T) {
	path := start(t, &Server{Handlers: echo(), Deadline: 300 * time.Millisecond})
	ctx := context.Background()
	var sum int
	if err := Call(ctx, path, "add", map[string]int{"A": 2, "B": 3}, &sum); err != nil || sum != 5 {
		t.Fatalf("%d %v", sum, err)
	}
	var ping struct{ Protocol int }
	if err := Call(ctx, path, "ping", nil, &ping); err != nil || ping.Protocol != Protocol {
		t.Errorf("%+v %v", ping, err)
	}
	for method, code := range map[string]int{"nope": CodeNoMethod, "fail": CodeServer, "boom": CodeServer, "slow": CodeServer} {
		err := Call(ctx, path, method, nil, nil)
		var e *Error
		if !errors.As(err, &e) || e.Code != code {
			t.Errorf("%s: %v", method, err)
		}
	}
	var e *Error
	if err := Call(ctx, path, "add", "not an object", nil); !errors.As(err, &e) || e.Code != CodeBadParams {
		t.Errorf("bad params: %v", err)
	}
	// The daemon survived the panicking handler.
	if err := Call(ctx, path, "ping", nil, nil); err != nil {
		t.Errorf("daemon died: %v", err)
	}
}

func raw(t *testing.T, path, send string) []string {
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(2 * time.Second))
	c.Write([]byte(send))
	var out []string
	sc := bufio.NewScanner(c)
	for sc.Scan() {
		out = append(out, sc.Text())
		if len(out) >= strings.Count(send, "\n") {
			break
		}
	}
	return out
}

func TestMalformedAndInvalidRequestsGetProperErrors(t *testing.T) {
	path := start(t, &Server{Handlers: echo()})
	got := raw(t, path, "{not json\n"+`{"jsonrpc":"1.0","id":1,"method":"ping"}`+"\n")
	if len(got) != 2 || !strings.Contains(got[0], "-32700") || !strings.Contains(got[1], "-32600") {
		t.Errorf("%q", got)
	}
}

func TestNotificationsGetNoAnswerAndUnknownFieldsAreIgnored(t *testing.T) {
	path := start(t, &Server{Handlers: echo()})
	got := raw(t, path, `{"jsonrpc":"2.0","method":"ping"}`+"\n"+`{"jsonrpc":"2.0","id":7,"method":"ping","from_the_future":true}`+"\n")
	if len(got) != 1 || !strings.Contains(got[0], `"id":7`) || !strings.Contains(got[0], `"protocol"`) {
		t.Errorf("%q", got)
	}
}

func TestOversizedRequestsAreRefused(t *testing.T) {
	path := start(t, &Server{Handlers: echo()})
	c, _ := net.Dial("unix", path)
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	go c.Write([]byte(strings.Repeat("x", 3<<20) + "\n"))
	line, _ := bufio.NewReader(c).ReadString('\n')
	if !strings.Contains(line, "too large") {
		t.Errorf("%q", line)
	}
}

func TestSocketIsPrivateAndAnotherUserIsDropped(t *testing.T) {
	path := start(t, &Server{Handlers: echo(), UID: os.Getuid() + 1}) // pretend only some other user may connect
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("socket mode: %v %v", st, err)
	}
	if err := Call(context.Background(), path, "ping", nil, nil); err == nil {
		t.Error("a connection from the wrong user was served")
	}
}

func TestListenReplacesAStaleSocketButNotALiveOne(t *testing.T) {
	path := start(t, &Server{Handlers: echo()})
	if _, err := Listen(path); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("a second daemon: %v", err)
	}
	stale := filepath.Join(t.TempDir(), "stale.sock")
	l, _ := net.Listen("unix", stale)
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close() // leaves the file behind, as a crash would
	if l2, err := Listen(stale); err != nil {
		t.Errorf("stale socket not replaced: %v", err)
	} else {
		l2.Close()
	}
}

func TestSocketPathPrefersTheRuntimeDirectory(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if SocketPath() != "/run/user/1000/safeshell.sock" {
		t.Error(SocketPath())
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	if !strings.HasPrefix(SocketPath(), "/tmp/safeshell-") {
		t.Error(SocketPath())
	}
}

func TestCallHonoursItsContext(t *testing.T) {
	path := start(t, &Server{Handlers: echo(), Deadline: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := Call(ctx, path, "slow", nil, nil); err == nil || time.Since(start) > 2*time.Second {
		t.Errorf("%v after %v", err, time.Since(start))
	}
}
