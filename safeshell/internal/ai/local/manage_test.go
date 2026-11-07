package local

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHasModel(t *testing.T) {
	have := []string{"qwen2.5-coder:3b", "llama3:latest"}
	if !HasModel(have, "qwen2.5-coder:3b") || !HasModel(have, "llama3") || HasModel(have, "phi3") {
		t.Error("model matching")
	}
}

func TestCompleteExplainsAMissingModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"models":[{"name":"other:1b"}]}`))
	}))
	defer srv.Close()
	_, err := Ollama{URL: srv.URL, Model: "qwen2.5-coder:3b"}.Complete(context.Background(), "", "")
	if err == nil || !strings.Contains(err.Error(), "ss ai setup") {
		t.Errorf("%v", err)
	}
}

func TestEnsureServerStartsItWhenDown(t *testing.T) {
	up := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !up {
			http.Error(w, "down", 503)
			return
		}
		w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()
	old := Starter
	defer func() { Starter = old }()
	started := 0
	Starter = func() error { started++; up = true; return nil }
	if err := (Ollama{URL: srv.URL}).EnsureServer(context.Background()); err != nil || started != 1 {
		t.Errorf("err %v started %d", err, started)
	}
	if err := (Ollama{URL: srv.URL}).EnsureServer(context.Background()); err != nil || started != 1 {
		t.Errorf("a running server must not be restarted (started %d)", started)
	}
}

func TestEnsureServerReportsWhenItCannotStart(t *testing.T) {
	old := Starter
	defer func() { Starter = old }()
	Starter = func() error { return errors.New("Ollama is not installed") }
	err := Ollama{URL: "http://127.0.0.1:1"}.EnsureServer(context.Background())
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("%v", err)
	}
}

func TestPullShowsProgressAndSurfacesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "bad") {
			w.Write([]byte("{\"error\":\"pull model manifest: not found\"}\n"))
			return
		}
		w.Write([]byte("{\"status\":\"pulling manifest\"}\n{\"status\":\"downloading\",\"total\":200,\"completed\":0}\n{\"status\":\"downloading\",\"total\":200,\"completed\":0,\"complete\":100}\n{\"status\":\"success\"}\n"))
	}))
	defer srv.Close()
	var out bytes.Buffer
	if err := (Ollama{URL: srv.URL, Model: "m"}).Pull(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "downloading 50%") || !strings.Contains(out.String(), "success") {
		t.Errorf("%q", out.String())
	}
	if err := (Ollama{URL: srv.URL + "/?bad", Model: "m"}).Pull(context.Background(), &out); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("%v", err)
	}
}
