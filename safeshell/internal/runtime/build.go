// Package runtime installs and runs the local model engine (llama.cpp's
// llama-server) so SafeShell needs nothing installed by hand. Downloads are
// pinned by SHA-256, resumable, and verified before use.
package runtime

import (
	"os"
	"path/filepath"
)

// Tag is the pinned llama.cpp release.
const Tag = "b11429"

// Build is one prebuilt llama.cpp archive.
type Build struct {
	Variant string // cpu or vulkan
	Asset   string
	Size    int64
	SHA     string
}

func (b Build) URL() string {
	return "https://github.com/ggml-org/llama.cpp/releases/download/" + Tag + "/" + b.Asset
}

var builds = map[string]Build{
	"amd64/cpu":    {"cpu", "llama-" + Tag + "-bin-ubuntu-x64.tar.gz", 17693462, "f6d25dde8f51133143d1453da4fd5f73b145127177612a283bf7995957af3392"},
	"amd64/vulkan": {"vulkan", "llama-" + Tag + "-bin-ubuntu-vulkan-x64.tar.gz", 31636673, "632c4e98feba2b94407a2130e3133e0c3aefb0ea1ab41337e926d8bfafdd0b74"},
	"arm64/cpu":    {"cpu", "llama-" + Tag + "-bin-ubuntu-arm64.tar.gz", 13682895, "ed44c0f79c3d02424f62bdc87e106318b0b89f49050e2e80c5f352723d14e00b"},
	"arm64/vulkan": {"vulkan", "llama-" + Tag + "-bin-ubuntu-vulkan-arm64.tar.gz", 24845090, "702d99c4219b4314cc6d3b10fb41ae96cbfc68226305c2681269dfb51487af34"},
}

var vulkanLibs = []string{
	"/usr/lib/x86_64-linux-gnu/libvulkan.so.1", "/usr/lib/aarch64-linux-gnu/libvulkan.so.1",
	"/usr/lib64/libvulkan.so.1", "/usr/lib/libvulkan.so.1",
}

// exists is replaced in tests.
var exists = func(p string) bool { _, err := os.Stat(p); return err == nil }

// HasVulkan reports whether the machine has a Vulkan loader, which is what a
// GPU build needs. Without one the CPU build is used.
func HasVulkan() bool {
	for _, p := range vulkanLibs {
		if exists(p) {
			return true
		}
	}
	return false
}

// Pick chooses the archive for an architecture. gpu is "auto", "on" or "off".
// ok is false when no build exists for the architecture.
func Pick(goarch, gpu string) (b Build, ok bool) {
	variant := "cpu"
	if gpu == "on" || (gpu != "off" && HasVulkan()) {
		variant = "vulkan"
	}
	b, ok = builds[goarch+"/"+variant]
	return b, ok
}

// Dirs says where things live.
type Dirs struct{ Data, State string }

// DefaultDirs follows the XDG spec.
func DefaultDirs() Dirs {
	home, _ := os.UserHomeDir()
	d := Dirs{Data: filepath.Join(home, ".local", "share", "safeshell"), State: filepath.Join(home, ".local", "state", "safeshell")}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		d.Data = filepath.Join(x, "safeshell")
	}
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		d.State = filepath.Join(x, "safeshell")
	}
	return d
}

// EngineDir is where one build is unpacked.
func (d Dirs) EngineDir(b Build) string { return filepath.Join(d.Data, "engine", Tag+"-"+b.Variant) }

// ServerBin is the llama-server executable of a build.
func (d Dirs) ServerBin(b Build) string { return filepath.Join(d.EngineDir(b), "llama-server") }

// ModelPath is where a catalog file is stored.
func (d Dirs) ModelPath(file string) string { return filepath.Join(d.Data, "models", file) }
