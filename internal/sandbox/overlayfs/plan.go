// Package overlayfs runs a command in rootless namespaces on top of an
// OverlayFS copy of the filesystem, so every write lands in a scratch layer
// and the real files are never touched. The scratch layers are then read back
// as an observed diff.
//
// The binary starts itself as `safeshell sandbox-init` inside the namespaces (the
// "re-exec" pattern): Go cannot configure namespaces between fork and exec.
package overlayfs

import (
	"path/filepath"
	"sort"
	"strings"
)

// Layer is one mounted filesystem that gets its own overlay.
type Layer struct {
	Mount string // where it is mounted on the host, "/" for the root filesystem
	Upper string // scratch directory receiving this layer's writes
	Work  string // overlay work directory, on the same filesystem as Upper
}

// overlayable filesystem types. Pseudo filesystems (proc, sysfs, devtmpfs,
// cgroup, ...) are rebuilt inside the sandbox instead, and read-only image
// types (squashfs, iso9660) are left out.
var overlayable = map[string]bool{
	"ext2": true, "ext3": true, "ext4": true, "xfs": true, "btrfs": true, "f2fs": true,
	"zfs": true, "vfat": true, "exfat": true, "ntfs": true, "ntfs3": true, "tmpfs": true,
}

// rebuilt are paths the sandbox mounts afresh, so host mounts under them are skipped.
var rebuilt = []string{"/proc", "/sys", "/dev", "/run"}

// layerMounts lists every path that gets its own overlay, shallowest first:
// the top-level directories of "/" plus every other real mounted filesystem
// (a separate /home, a USB disk, ...). The kernel refuses "/" itself as a lower
// layer, so the root is rebuilt from these pieces instead.
func layerMounts(topDirs []string, mountinfo string) []string {
	seen := map[string]bool{}
	for _, d := range topDirs {
		seen["/"+d] = true
	}
	for _, m := range mountPoints(mountinfo) {
		seen[m] = true
	}
	var out []string
	for m := range seen {
		out = append(out, m)
	}
	sortShallowFirst(out)
	return out
}

// isSpecialTop names the top-level directories the sandbox rebuilds itself.
func isSpecialTop(name string) bool {
	switch name {
	case "proc", "sys", "dev", "run":
		return true
	}
	return false
}

// mountPoints lists the real mounted filesystems other than "/", shallowest
// first. mountinfo is the text of /proc/self/mountinfo.
func mountPoints(mountinfo string) []string {
	seen := map[string]bool{}
	for _, line := range strings.Split(mountinfo, "\n") {
		before, after, ok := strings.Cut(line, " - ")
		if !ok {
			continue
		}
		f := strings.Fields(before)
		a := strings.Fields(after)
		if len(f) < 5 || len(a) < 1 || !overlayable[a[0]] {
			continue
		}
		mp := unescape(f[4])
		if mp == "/" || underAny(mp, rebuilt) {
			continue
		}
		seen[mp] = true
	}
	var out []string
	for mp := range seen {
		out = append(out, mp)
	}
	sortShallowFirst(out)
	return out
}

func sortShallowFirst(ps []string) {
	sort.Slice(ps, func(i, j int) bool {
		di, dj := strings.Count(ps[i], "/"), strings.Count(ps[j], "/")
		if di != dj {
			return di < dj
		}
		return ps[i] < ps[j]
	})
}

func underAny(p string, roots []string) bool {
	for _, r := range roots {
		if p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

// unescape decodes the octal escapes mountinfo uses for spaces and tabs.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			n := 0
			ok := true
			for _, c := range s[i+1 : i+4] {
				if c < '0' || c > '7' {
					ok = false
					break
				}
				n = n*8 + int(c-'0')
			}
			if ok {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// plan lays out one Layer per mount below scratch.
func plan(mounts []string, scratch string) []Layer {
	layers := make([]Layer, len(mounts))
	for i, m := range mounts {
		dir := filepath.Join(scratch, "layers", itoa(i))
		layers[i] = Layer{Mount: m, Upper: filepath.Join(dir, "upper"), Work: filepath.Join(dir, "work")}
	}
	return layers
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var d []byte
	for ; i > 0; i /= 10 {
		d = append([]byte{byte('0' + i%10)}, d...)
	}
	return string(d)
}
