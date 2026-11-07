// Package local runs the model on this machine through the managed engine.
package local

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	goruntime "runtime"
	"time"

	"safeshell/internal/ai/openai"
	"safeshell/internal/models"
	"safeshell/internal/runtime"
)

// Llama is a catalog model served by the managed llama-server.
type Llama struct {
	Dirs  runtime.Dirs
	Model models.Model
	GPU   string    // auto, on, off
	Log   io.Writer // where "starting the model" notices go; may be nil
}

func (l Llama) Name() string { return "local:" + l.Model.ID }

// Ready reports whether the engine and model files are installed.
func (l Llama) Ready() bool {
	b, ok := runtime.Pick(goruntime.GOARCH, l.GPU)
	return ok && l.Dirs.EngineInstalled(b) && l.Dirs.ModelInstalled(l.Model)
}

func (l Llama) Complete(ctx context.Context, system, user string) (string, error) {
	b, ok := runtime.Pick(goruntime.GOARCH, l.GPU)
	if !ok {
		return "", fmt.Errorf("no local engine is available for %s; use a cloud model (/models)", goruntime.GOARCH)
	}
	if !l.Ready() {
		return "", errors.New("the local model is not downloaded yet: type /models or run  ss ai setup")
	}
	port, err := l.Dirs.Ensure(ctx, runtime.Spec{Bin: l.Dirs.ServerBin(b), Model: l.Dirs.ModelPath(l.Model.File), GPU: b.Variant == "vulkan"}, l.Log)
	if err != nil {
		return "", err
	}
	zero := 0.0
	return openai.Client{
		Label: l.Name(), URL: fmt.Sprintf("http://127.0.0.1:%d/v1", port), Model: l.Model.ID, Temperature: &zero,
		Client: &http.Client{Timeout: 120 * time.Second},
	}.Complete(ctx, system, user)
}
