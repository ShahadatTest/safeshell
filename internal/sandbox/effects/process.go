// Package effects inspects the side effects a file sandbox cannot see. Each
// inspector only reads the real system (never changes it) and reports an
// Effect with an evidence level.
package effects

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/samarthsinh2660/safeshell/internal/domain"
	"github.com/samarthsinh2660/safeshell/internal/shellparse"
)

// Proc is one running process.
type Proc struct {
	PID, PPID, UID int
	Comm, Cmdline  string
}

// ProcFS reads process information from a /proc-shaped directory.
type ProcFS struct{ Root string }

// List returns every process that has a command line; kernel threads have none.
func (p ProcFS) List() []Proc {
	entries, _ := os.ReadDir(p.Root)
	var out []Proc
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if pr, ok := p.read(pid); ok {
			out = append(out, pr)
		}
	}
	return out
}

func (p ProcFS) read(pid int) (Proc, bool) {
	dir := filepath.Join(p.Root, strconv.Itoa(pid))
	stat, err := os.ReadFile(filepath.Join(dir, "stat"))
	if err != nil {
		return Proc{}, false
	}
	comm, ppid, ok := parseStat(string(stat))
	if !ok {
		return Proc{}, false
	}
	cmd, _ := os.ReadFile(filepath.Join(dir, "cmdline"))
	line := strings.TrimSpace(strings.ReplaceAll(string(cmd), "\x00", " "))
	if line == "" {
		return Proc{}, false
	}
	uid := -1
	if fi, err := os.Stat(dir); err == nil {
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			uid = int(st.Uid)
		}
	}
	return Proc{PID: pid, PPID: ppid, UID: uid, Comm: comm, Cmdline: line}, true
}

// parseStat reads "pid (comm) state ppid ..." where comm may contain spaces and parentheses.
func parseStat(s string) (comm string, ppid int, ok bool) {
	open, close := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open < 0 || close < open {
		return "", 0, false
	}
	rest := strings.Fields(s[close+1:])
	if len(rest) < 2 {
		return "", 0, false
	}
	ppid, err := strconv.Atoi(rest[1])
	return s[open+1 : close], ppid, err == nil
}

// Ancestors returns pid's parent chain (parent first), so the user's own shell
// can be recognised as a target.
func (p ProcFS) Ancestors(pid int) []int {
	var out []int
	seen := map[int]bool{pid: true}
	for {
		pr, ok := p.read(pid)
		if !ok || pr.PPID <= 0 || seen[pr.PPID] {
			return out
		}
		out = append(out, pr.PPID)
		seen[pr.PPID] = true
		pid = pr.PPID
	}
}

// criticalComms are processes whose death logs you out, cuts the machine off
// or freezes the desktop.
var criticalComms = map[string]bool{
	"systemd": true, "init": true, "sshd": true, "dbus-daemon": true, "Xorg": true, "Xwayland": true,
	"gnome-shell": true, "kwin_x11": true, "kwin_wayland": true, "NetworkManager": true, "gdm": true, "sddm": true, "lightdm": true,
}

// InspectProcesses finds the real processes that kill, pkill and killall would
// signal. self is the pid of the calling process (never a target); uid is the
// current user.
func InspectProcesses(fs ProcFS, self, uid int, cmds []shellparse.Command) []domain.Effect {
	var out []domain.Effect
	var procs []Proc
	loaded := false
	load := func() []Proc {
		if !loaded {
			procs, loaded = fs.List(), true
		}
		return procs
	}
	protected := map[int]bool{self: true}
	for _, a := range fs.Ancestors(self) {
		protected[a] = true
	}
	for _, c := range cmds {
		switch c.Name {
		case "kill":
			out = append(out, pidTargets(load(), c, protected, uid)...)
		case "pkill", "killall":
			out = append(out, patternTargets(load(), c, self, protected, uid)...)
		}
	}
	return out
}

func pidTargets(all []Proc, c shellparse.Command, protected map[int]bool, uid int) []domain.Effect {
	var hit []Proc
	for _, a := range c.Args {
		pid, err := strconv.Atoi(a)
		if err != nil || pid <= 0 {
			continue
		}
		for _, p := range all {
			if p.PID == pid {
				hit = append(hit, p)
			}
		}
	}
	if len(hit) == 0 {
		return nil
	}
	return []domain.Effect{effect(hit, protected, uid, "")}
}

func patternTargets(all []Proc, c shellparse.Command, self int, protected map[int]bool, uid int) []domain.Effect {
	var pattern string
	full, exact := false, c.Name == "killall"
	for i := 0; i < len(c.Args); i++ {
		a := c.Args[i]
		switch {
		case a == "-f" || a == "--full":
			full = true
		case a == "-x" || a == "--exact":
			exact = true
		case valueOptions[a]:
			i++ // the next word is this option's value, not the pattern
		case strings.HasPrefix(a, "-"):
		case pattern == "":
			pattern = a
		}
	}
	if pattern == "" {
		return nil
	}
	expr := pattern
	if exact {
		expr = "^(?:" + pattern + ")$"
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return []domain.Effect{{Kind: domain.EffectProcess, Evidence: domain.Estimated, Summary: fmt.Sprintf("the pattern %q could not be evaluated, so the targets are unknown", pattern), MinLevel: domain.R2}}
	}
	var hit []Proc
	for _, p := range all {
		if p.PID == self {
			continue
		}
		text := p.Comm
		if full {
			text = p.Cmdline
		}
		if re.MatchString(text) {
			hit = append(hit, p)
		}
	}
	note := ""
	if len(c.Args) > 0 && hasUserFilter(c.Args) {
		note = " (the -u user filter was not applied, so this may list too many)"
	}
	if len(hit) == 0 {
		return []domain.Effect{{Kind: domain.EffectProcess, Evidence: domain.Inspected, Summary: fmt.Sprintf("no running process matches %q", pattern)}}
	}
	return []domain.Effect{effect(hit, protected, uid, note)}
}

// valueOptions of pkill and killall that take an argument.
var valueOptions = map[string]bool{"-u": true, "-U": true, "-g": true, "-G": true, "-P": true, "-s": true, "-t": true, "-n": true}

func hasUserFilter(args []string) bool {
	for _, a := range args {
		if a == "-u" || a == "--euid" || a == "-U" || a == "--uid" {
			return true
		}
	}
	return false
}

func effect(hit []Proc, protected map[int]bool, uid int, note string) domain.Effect {
	e := domain.Effect{Kind: domain.EffectProcess, Evidence: domain.Inspected, MinLevel: domain.R2}
	var critical []string
	for _, p := range hit {
		line := fmt.Sprintf("%s (pid %d, %s): %s", p.Comm, p.PID, owner(p.UID, uid), clip(p.Cmdline, 70))
		e.Details = append(e.Details, line)
		switch {
		case p.PID == 1 || criticalComms[p.Comm]:
			critical = append(critical, fmt.Sprintf("%s (pid %d)", p.Comm, p.PID))
		case protected[p.PID]:
			critical = append(critical, fmt.Sprintf("%s (pid %d, your own shell or its parent)", p.Comm, p.PID))
		}
	}
	e.Summary = fmt.Sprintf("would signal %d running process(es)%s", len(hit), note)
	switch {
	case len(critical) > 0:
		e.MinLevel = domain.R4
		e.Summary += ", including " + strings.Join(critical, ", ") + ", which would log you out or take the machine down"
	case len(hit) >= 20:
		e.MinLevel = domain.R3
	}
	return e
}

func owner(procUID, me int) string {
	switch procUID {
	case me:
		return "yours"
	case 0:
		return "root"
	case -1:
		return "unknown owner"
	}
	return "uid " + strconv.Itoa(procUID)
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
