package runtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"safeshell/internal/models"
)

// EngineInstalled reports whether the build is unpacked and runnable.
func (d Dirs) EngineInstalled(b Build) bool {
	fi, err := os.Stat(d.ServerBin(b))
	return err == nil && fi.Mode()&0o111 != 0
}

// ModelInstalled reports whether the model file is present and complete.
func (d Dirs) ModelInstalled(m models.Model) bool {
	fi, err := os.Stat(d.ModelPath(m.File))
	return err == nil && fi.Size() == m.Size
}

// InstallEngine downloads, verifies and unpacks a build. It does nothing if
// the build is already installed.
func (d Dirs) InstallEngine(ctx context.Context, client *http.Client, b Build, w io.Writer) error {
	if d.EngineInstalled(b) {
		return nil
	}
	archive := filepath.Join(d.Data, "downloads", b.Asset)
	fmt.Fprintf(w, "Downloading the local AI engine (llama.cpp %s, %s, %s)...\n", Tag, b.Variant, mb(b.Size))
	if err := Download(ctx, client, b.URL(), archive, b.SHA, b.Size, w); err != nil {
		return err
	}
	dest := d.EngineDir(b)
	tmp := dest + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := ExtractTarGz(archive, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	_ = os.RemoveAll(dest)
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	_ = os.Remove(archive)
	return nil
}

// InstallModel downloads and verifies a catalog model.
func (d Dirs) InstallModel(ctx context.Context, client *http.Client, m models.Model, w io.Writer) error {
	if d.ModelInstalled(m) {
		return nil
	}
	fmt.Fprintf(w, "Downloading %s (%s)...\n", m.Title, mb(m.Size))
	return Download(ctx, client, m.URL, d.ModelPath(m.File), m.SHA, m.Size, w)
}
