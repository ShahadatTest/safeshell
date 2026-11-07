// Package local talks to an Ollama server on this machine.
package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Ollama is a local model server. Nothing leaves the machine.
type Ollama struct {
	URL    string // default http://127.0.0.1:11434
	Model  string
	Client *http.Client
}

func (o Ollama) Name() string { return "local:" + o.Model }

func (o Ollama) Complete(ctx context.Context, system, user string) (string, error) {
	if err := o.EnsureServer(ctx); err != nil {
		return "", err
	}
	if have, err := o.Models(ctx); err == nil && !HasModel(have, o.Model) {
		return "", fmt.Errorf("the model %s is not downloaded yet: run  ss ai setup", o.Model)
	}
	base := o.base()
	body, _ := json.Marshal(map[string]any{
		"model": o.Model, "stream": false, "keep_alive": "30m", "options": map[string]any{"temperature": 0},
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c := o.Client
	if c == nil {
		c = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama is not reachable at %s: %w", base, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama answered %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Message struct{ Content string } `json:"message"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.Message.Content == "" {
		return "", fmt.Errorf("Ollama sent no usable reply")
	}
	return out.Message.Content, nil
}
