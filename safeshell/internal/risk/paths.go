package risk

import (
	"os"
	"strings"
)

var systemDirs = []string{"/etc", "/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/boot", "/var", "/opt", "/sys", "/proc", "/dev", "/root", "/home", "/srv", "/run"}

// normPath makes common spellings of the same location comparable.
func normPath(p string) string {
	p = strings.ReplaceAll(p, "${HOME}", "~")
	if strings.HasPrefix(p, "$HOME") {
		p = "~" + strings.TrimPrefix(p, "$HOME")
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
		if p == "" {
			p = "/"
		}
	}
	return p
}

// critical reports whether deleting or rewriting p would damage the whole
// system or the whole home directory, and says what p is.
func critical(raw string) (bool, string) {
	p := normPath(raw)
	base := strings.TrimSuffix(p, "/*")
	if base == "" {
		base = "/"
	}
	if base == "/" {
		return true, "the entire filesystem"
	}
	if base == "~" || (home() != "" && base == home()) {
		return true, "your whole home directory"
	}
	for _, d := range systemDirs {
		if base == d {
			return true, "the system directory " + d
		}
	}
	if strings.HasPrefix(base, "/home/") && strings.Count(strings.Trim(base, "/"), "/") == 1 {
		return true, "a user's whole home directory"
	}
	return false, ""
}

func underSystem(raw string) bool {
	p := normPath(raw)
	for _, d := range systemDirs {
		if strings.HasPrefix(p, d+"/") && d != "/home" && d != "/dev" && d != "/run" && d != "/var" {
			return true
		}
	}
	return false
}

// wholeCwd reports a target that means "everything in the current directory".
func wholeCwd(p string) bool {
	switch normPath(p) {
	case ".", "*", "./*", "..", "../*", ".*", "./.*":
		return true
	}
	return false
}

func isBlockDevice(p string) bool {
	for _, pre := range []string{"/dev/sd", "/dev/hd", "/dev/vd", "/dev/xvd", "/dev/nvme", "/dev/mmcblk", "/dev/disk", "/dev/mapper", "/dev/md", "/dev/loop"} {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

// shq quotes s for display only if it needs it.
func shq(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"'$`\\!&;|<>(){}") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
