package effects

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/samarthsinh2660/safeshell/internal/domain"
	"github.com/samarthsinh2660/safeshell/internal/report"
	"github.com/samarthsinh2660/safeshell/internal/shellparse"
)

// SysFS locates the files used to identify a block device.
type SysFS struct {
	Dev       string // "/dev"
	Sys       string // "/sys"
	Mountinfo string // the text of /proc/self/mountinfo
}

// diskWriters are programs that overwrite or reformat the device they are given.
var diskWriters = map[string]bool{
	"mkswap": true, "wipefs": true, "fdisk": true, "sfdisk": true, "gdisk": true, "parted": true,
	"blkdiscard": true, "shred": true, "srm": true, "dd": true,
}

var diskLike = []string{"sd", "hd", "vd", "xvd", "nvme", "mmcblk", "loop", "dm-", "md"}

// systemMounts mark a device as holding the running system.
var systemMounts = map[string]bool{"/": true, "/boot": true, "/boot/efi": true, "/usr": true, "/var": true, "/home": true}

// InspectDisks identifies the block devices a command would write to and says
// what is on them, mounted where, and whether the system runs from them.
func InspectDisks(fs SysFS, cmds []shellparse.Command) []domain.Effect {
	var out []domain.Effect
	seen := map[string]bool{}
	for _, c := range cmds {
		for _, dev := range deviceTargets(c) {
			if seen[dev] {
				continue
			}
			seen[dev] = true
			if e, ok := inspectDevice(fs, dev); ok {
				out = append(out, e)
			}
		}
	}
	return out
}

// deviceTargets lists the /dev paths c would write to.
func deviceTargets(c shellparse.Command) []string {
	var out []string
	add := func(p string) {
		if strings.HasPrefix(p, "/dev/") {
			out = append(out, p)
		}
	}
	name := c.Name
	if strings.HasPrefix(name, "mkfs.") {
		name = "mkfs"
	}
	if name == "mkfs" || diskWriters[name] {
		for _, a := range c.Args {
			if v, ok := strings.CutPrefix(a, "of="); ok {
				add(v)
			} else if !strings.HasPrefix(a, "-") {
				add(a)
			}
		}
	}
	for _, r := range c.Redirs {
		if strings.HasPrefix(r.Op, ">") || strings.HasPrefix(r.Op, "&>") {
			add(r.Target)
		}
	}
	return out
}

func inspectDevice(fs SysFS, dev string) (domain.Effect, bool) {
	local := filepath.Join(fs.Dev, strings.TrimPrefix(dev, "/dev/"))
	resolved, err := filepath.EvalSymlinks(local)
	if err != nil {
		resolved = local
	}
	name := filepath.Base(resolved)
	if !looksLikeDisk(name) {
		return domain.Effect{}, false
	}
	real, err := filepath.EvalSymlinks(filepath.Join(fs.Sys, "class", "block", name))
	if err != nil {
		return domain.Effect{
			Kind: domain.EffectDisk, Evidence: domain.Estimated, MinLevel: domain.R4,
			Summary: fmt.Sprintf("would write to %s, but it could not be identified, so its contents are unknown", dev),
		}, true
	}
	diskDir := real
	isPart := exists(filepath.Join(real, "partition"))
	if isPart {
		diskDir = filepath.Dir(real)
	}
	parts := partitions(diskDir)
	devnums := map[string]bool{}
	if isPart {
		devnums[readTrim(filepath.Join(real, "dev"))] = true
	} else {
		devnums[readTrim(filepath.Join(diskDir, "dev"))] = true
		for _, p := range parts {
			devnums[readTrim(filepath.Join(diskDir, p, "dev"))] = true
		}
	}
	mounts := mountpoints(fs.Mountinfo, devnums)

	what := "disk"
	if isPart {
		what = "partition"
	}
	sectors, _ := strconv.ParseInt(readTrim(filepath.Join(real, "size")), 10, 64)
	e := domain.Effect{Kind: domain.EffectDisk, Evidence: domain.Inspected, MinLevel: domain.R4}
	e.Summary = fmt.Sprintf("would overwrite or erase %s, a %s of %s", dev, what, report.HumanBytes(sectors*512))
	if !isPart {
		e.Details = append(e.Details, fmt.Sprintf("%d partition(s): %s", len(parts), strings.Join(parts, ", ")))
	}
	if len(mounts) == 0 {
		e.Details = append(e.Details, "not mounted")
	} else {
		e.Details = append(e.Details, "mounted at "+strings.Join(mounts, ", "))
		for _, m := range mounts {
			if systemMounts[m] {
				e.Summary += ". It holds the running system"
				break
			}
		}
	}
	return e, true
}

func looksLikeDisk(name string) bool {
	for _, p := range diskLike {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func partitions(diskDir string) []string {
	entries, _ := os.ReadDir(diskDir)
	var out []string
	for _, e := range entries {
		if e.IsDir() && exists(filepath.Join(diskDir, e.Name(), "partition")) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// mountpoints finds where any of the device numbers (major:minor) are mounted.
func mountpoints(mountinfo string, devnums map[string]bool) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(mountinfo, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !devnums[f[2]] || seen[f[4]] {
			continue
		}
		seen[f[4]] = true
		out = append(out, f[4])
	}
	sort.Strings(out)
	return out
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func readTrim(p string) string {
	b, _ := os.ReadFile(p)
	return strings.TrimSpace(string(b))
}
