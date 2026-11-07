package local

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func (o Ollama) base() string {
	if b := strings.TrimRight(o.URL, "/"); b != "" {
		return b
	}
	return "http://127.0.0.1:11434"
}

func (o Ollama) client(d time.Duration) *http.Client {
	if o.Client != nil {
		return o.Client
	}
	return &http.Client{Timeout: d}
}

// Models lists the models the server has downloaded. An error means the server is not reachable.
func (o Ollama) Models(ctx context.Context) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, o.base()+"/api/tags", nil)
	resp, err := o.client(3 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct{ Models []struct{ Name string } }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range out.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

// HasModel reports whether the wanted model is downloaded ("name" matches "name:latest").
func HasModel(have []string, want string) bool {
	for _, h := range have {
		if h == want || h == want+":latest" {
			return true
		}
	}
	return false
}

// Starter launches the model server; tests replace it.
var Starter = func() error {
	bin, err := exec.LookPath("ollama")
	if err != nil {
		return errors.New("Ollama is not installed (https://ollama.com/download)")
	}
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("safeshell-ollama-%d.log", os.Getuid()))
	log, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	cmd := exec.Command(bin, "serve")
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // keeps running after the shell command ends
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// EnsureServer makes sure the model server answers, starting it if needed.
func (o Ollama) EnsureServer(ctx context.Context) error {
	if _, err := o.Models(ctx); err == nil {
		return nil
	}
	if err := Starter(); err != nil {
		return err
	}
	for end := time.Now().Add(20 * time.Second); time.Now().Before(end); {
		if _, err := o.Models(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return errors.New("started Ollama but it did not answer within 20 seconds")
}

// Pull downloads the model, reporting progress to w.
func (o Ollama) Pull(ctx context.Context, w io.Writer) error {
	body, _ := json.Marshal(map[string]any{"model": o.Model, "stream": true})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, o.base()+"/api/pull", strings.NewReader(string(body)))
	resp, err := o.client(0).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama answered %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	last := ""
	for sc.Scan() {
		var m struct {
			Status          string
			Error           string
			Total, Complete int64
		}
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		if m.Error != "" {
			return errors.New(m.Error)
		}
		line := m.Status
		if m.Total > 0 {
			line = fmt.Sprintf("%s %d%%", m.Status, m.Complete*100/m.Total)
		}
		if line != last {
			fmt.Fprintf(w, "\r%-60s", line)
			last = line
		}
	}
	fmt.Fprintln(w)
	return sc.Err()
}
