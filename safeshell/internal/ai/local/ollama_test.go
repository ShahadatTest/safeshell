package local

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaSendsChatAndReturnsContent(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Write([]byte(`{"models":[{"name":"qwen"},{"name":"x"}]}`))
			return
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("path %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"message":{"role":"assistant","content":"hello"}}`))
	}))
	defer srv.Close()
	out, err := Ollama{URL: srv.URL, Model: "qwen"}.Complete(context.Background(), "sys", "usr")
	if err != nil || out != "hello" {
		t.Fatalf("%q %v", out, err)
	}
	if got["model"] != "qwen" || got["stream"] != false {
		t.Errorf("request %v", got)
	}
}

func TestOllamaErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.Write([]byte(`{"models":[{"name":"x"}]}`))
			return
		}
		if r.URL.Query().Get("empty") != "" {
			w.Write([]byte(`{}`))
			return
		}
		http.Error(w, "model not found", 404)
	}))
	defer srv.Close()
	if _, err := (Ollama{URL: srv.URL, Model: "x"}).Complete(context.Background(), "", ""); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("%v", err)
	}
	if _, err := (Ollama{URL: srv.URL + "/?empty=1", Model: "x"}).Complete(context.Background(), "", ""); err == nil {
		t.Error("empty reply")
	}
	old := Starter
	defer func() { Starter = old }()
	Starter = func() error { return errors.New("Ollama is not installed") }
	if _, err := (Ollama{URL: "http://127.0.0.1:1", Model: "x"}).Complete(context.Background(), "", ""); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("%v", err)
	}
}
