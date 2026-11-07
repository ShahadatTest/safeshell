package runtime

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"safeshell/internal/models"
)

// TestMain lets the test binary stand in for llama-server: when started with
// SS_FAKE_LLAMA it serves /health on the --port it is given.
func TestMain(m *testing.M) {
	if os.Getenv("SS_FAKE_LLAMA") != "" {
		for i, a := range os.Args {
			if a == "--port" {
				port, _ := strconv.Atoi(os.Args[i+1])
				if os.Getenv("SS_FAKE_LLAMA") == "crash" {
					os.Exit(3)
				}
				mux := http.NewServeMux()
				mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"status":"ok"}`)) })
				_ = http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), mux)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func payload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 7)
	}
	return b
}

func TestDownloadVerifiesAndResumes(t *testing.T) {
	data := payload(200_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", time.Time{}, strings.NewReader(string(data)))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "d", "f.bin")
	// A previous interrupted run left the first 50 kB.
	os.MkdirAll(filepath.Dir(dest), 0o755)
	os.WriteFile(dest+".part", data[:50_000], 0o644)
	var out strings.Builder
	if err := Download(context.Background(), nil, srv.URL, dest, sum(data), int64(len(data)), &out); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != string(data) {
		t.Error("content differs")
	}
	if _, err := os.Stat(dest + ".part"); err == nil {
		t.Error("partial file left behind")
	}
	if !strings.Contains(out.String(), "100%") {
		t.Errorf("progress: %q", out.String())
	}
	// A second call finds the verified file and does not touch the network.
	srv.Close()
	if err := Download(context.Background(), nil, srv.URL, dest, sum(data), int64(len(data)), nil); err != nil {
		t.Errorf("already downloaded: %v", err)
	}
}

func TestDownloadRejectsAndDeletesCorruptFiles(t *testing.T) {
	data := payload(10_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	err := Download(context.Background(), nil, srv.URL, dest, sum([]byte("something else")), int64(len(data)), nil)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err %v", err)
	}
	for _, p := range []string{dest, dest + ".part"} {
		if _, e := os.Stat(p); e == nil {
			t.Errorf("%s was kept", p)
		}
	}
}

func TestDownloadStartsOverWhenTheServerIgnoresRanges(t *testing.T) {
	data := payload(20_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) })) // always 200
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	os.WriteFile(dest+".part", data[:5_000], 0o644)
	if err := Download(context.Background(), nil, srv.URL, dest, sum(data), int64(len(data)), nil); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if err := Download(context.Background(), nil, srv.URL, filepath.Join(t.TempDir(), "x"), "00", 10, nil); err == nil {
		t.Error("404 accepted")
	}
}

type entry struct {
	name, link, body string
	typ              byte
	mode             int64
}

func archive(t *testing.T, entries ...entry) string {
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	f, _ := os.Create(p)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		tw.WriteHeader(&tar.Header{Name: e.name, Linkname: e.link, Typeflag: e.typ, Mode: e.mode, Size: int64(len(e.body))})
		tw.Write([]byte(e.body))
	}
	tw.Close()
	gz.Close()
	f.Close()
	return p
}

func TestExtractStripsTopFolderKeepsModesAndLinks(t *testing.T) {
	a := archive(t,
		entry{name: "llama-b1/", typ: tar.TypeDir, mode: 0o755},
		entry{name: "llama-b1/llama-server", typ: tar.TypeReg, mode: 0o755, body: "bin"},
		entry{name: "llama-b1/libx.so.0", typ: tar.TypeReg, mode: 0o644, body: "lib"},
		entry{name: "llama-b1/libx.so", typ: tar.TypeSymlink, link: "libx.so.0"},
	)
	dest := filepath.Join(t.TempDir(), "e")
	if err := ExtractTarGz(a, dest); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(dest, "llama-server")); err != nil || fi.Mode()&0o111 == 0 {
		t.Errorf("server not executable: %v %v", fi, err)
	}
	if l, _ := os.Readlink(filepath.Join(dest, "libx.so")); l != "libx.so.0" {
		t.Errorf("link %q", l)
	}
}

func TestExtractRefusesEscapes(t *testing.T) {
	for name, e := range map[string]entry{
		"dotdot":        {name: "top/../../evil", typ: tar.TypeReg, mode: 0o644, body: "x"},
		"abs symlink":   {name: "top/l", typ: tar.TypeSymlink, link: "/etc/passwd"},
		"up symlink":    {name: "top/l", typ: tar.TypeSymlink, link: "../../outside"},
		"nested dotdot": {name: "top/a/../../../evil", typ: tar.TypeReg, mode: 0o644, body: "x"},
	} {
		dest := filepath.Join(t.TempDir(), "e")
		if err := ExtractTarGz(archive(t, e), dest); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestPickPrefersVulkanOnlyWhenAvailable(t *testing.T) {
	old := exists
	defer func() { exists = old }()
	exists = func(string) bool { return false }
	if b, ok := Pick("amd64", "auto"); !ok || b.Variant != "cpu" {
		t.Errorf("%+v", b)
	}
	if b, _ := Pick("amd64", "on"); b.Variant != "vulkan" {
		t.Error("gpu on")
	}
	exists = func(p string) bool { return strings.HasSuffix(p, "libvulkan.so.1") }
	if b, _ := Pick("amd64", "auto"); b.Variant != "vulkan" {
		t.Error("auto with vulkan")
	}
	if b, _ := Pick("amd64", "off"); b.Variant != "cpu" {
		t.Error("gpu off")
	}
	if _, ok := Pick("riscv64", "auto"); ok {
		t.Error("unsupported arch")
	}
	for _, b := range builds {
		if len(b.SHA) != 64 || !strings.HasPrefix(b.URL(), "https://github.com/ggml-org/llama.cpp/releases/download/"+Tag) {
			t.Errorf("bad build %+v", b)
		}
	}
}

func TestInstallEngineAndModelEndToEndWithFakeServers(t *testing.T) {
	arch := archive(t, entry{name: "t/llama-server", typ: tar.TypeReg, mode: 0o755, body: "bin"})
	ab, _ := os.ReadFile(arch)
	model := payload(30_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".tar.gz") {
			w.Write(ab)
		} else {
			w.Write(model)
		}
	}))
	defer srv.Close()
	d := Dirs{Data: t.TempDir(), State: t.TempDir()}
	b := Build{Variant: "cpu", Asset: "x.tar.gz", Size: int64(len(ab)), SHA: sum(ab)}
	// Point the download at the fake server by overriding URL via a client transport.
	client := &http.Client{Transport: rewrite{srv.URL}}
	if err := d.InstallEngine(context.Background(), client, b, &strings.Builder{}); err != nil || !d.EngineInstalled(b) {
		t.Fatalf("engine: %v", err)
	}
	m := models.Model{Title: "T", File: "m.gguf", URL: "https://example.invalid/m.gguf", Size: int64(len(model)), SHA: sum(model)}
	if d.ModelInstalled(m) {
		t.Fatal("model reported installed too early")
	}
	if err := d.InstallModel(context.Background(), client, m, &strings.Builder{}); err != nil || !d.ModelInstalled(m) {
		t.Fatalf("model: %v", err)
	}
}

type rewrite struct{ to string }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.Path
	req2, _ := http.NewRequest(req.Method, r.to+u, nil)
	req2.Header = req.Header
	return http.DefaultTransport.RoundTrip(req2)
}

func TestEnsureStartsOnceReusesAndRestartsForAnotherModel(t *testing.T) {
	t.Setenv("SS_FAKE_LLAMA", "1")
	d := Dirs{Data: t.TempDir(), State: t.TempDir()}
	t.Cleanup(func() { d.Stop() })
	spec := Spec{Bin: os.Args[0], Model: "/m/a.gguf"}
	p1, err := d.Ensure(context.Background(), spec, nil)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := d.Ensure(context.Background(), spec, nil)
	if err != nil || p2 != p1 {
		t.Errorf("a healthy server must be reused: %d %d %v", p1, p2, err)
	}
	if m, ok := d.Running(); !ok || m != "/m/a.gguf" {
		t.Errorf("running: %q %v", m, ok)
	}
	spec.Model = "/m/b.gguf"
	p3, err := d.Ensure(context.Background(), spec, nil)
	if err != nil || p3 == p1 {
		t.Errorf("another model needs a new server: %d %d %v", p1, p3, err)
	}
	if !d.Stop() {
		t.Error("stop")
	}
	if _, ok := d.Running(); ok {
		t.Error("still running after stop")
	}
}

func TestEnsureReportsAnEngineThatCrashes(t *testing.T) {
	t.Setenv("SS_FAKE_LLAMA", "crash")
	d := Dirs{Data: t.TempDir(), State: t.TempDir()}
	_, err := d.Ensure(context.Background(), Spec{Bin: os.Args[0], Model: "/m/a.gguf"}, nil)
	if err == nil || !strings.Contains(err.Error(), "llama.log") {
		t.Errorf("%v", err)
	}
	if _, err := d.Ensure(context.Background(), Spec{Bin: "/nonexistent/llama-server"}, nil); err == nil {
		t.Error("missing binary")
	}
}
