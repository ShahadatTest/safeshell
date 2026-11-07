// Command ss is the SafeShell command-line tool.
//
// Phase 1 provides `ss check`, which rates a command without running it.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"safeshell/internal/classify"
	"safeshell/internal/domain"
	"safeshell/internal/platform"
	"safeshell/internal/report"
	"safeshell/internal/sandbox"
	"safeshell/internal/sandbox/overlayfs"
)

// probe and simulate are replaced in tests.
var (
	probe    = platform.Probe
	simulate = overlayfs.Run
)

const usage = `SafeShell: a safety layer for your terminal.

Usage:
  ss check [--json] <command...>   rate a command without running it
  ss analyze [--json] [--cwd DIR] <command...>
                                   the full picture: risk level, a sandbox run, the real
                                   processes and disks it would hit, and safer options
  ss simulate [--json] [--cwd DIR] [--timeout 30s] <command...>
                                   run a command in a sandbox and show what it changed;
                                   the real files are never touched
  ss guard [--cwd DIR] -- <command...>
                                   what the shell hook runs: analyse, ask, print the command to run
  ss undo [ID]                     put back what the last approved command changed, and verify it
  ss snapshots                     list saved snapshots
  ss hook bash|zsh                 print the shell hook; add  eval "$(ss hook bash)"  to your rc file
  ss doctor                        show what this machine allows (sandbox, snapshots, AI)

Quote the command so your shell passes it through untouched:
  ss check 'rm -rf ~/projects'

Exit status of 'ss check': 0 = R0-R2, 3 = R3 (irreversible), 4 = R4 (catastrophic).
`

func main() {
	// The sandbox re-executes this binary; that path must run before anything else.
	if len(os.Args) > 1 && os.Args[1] == "sandbox-init" {
		os.Exit(overlayfs.Init())
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, useColor(os.Stdout)))
}

// run is main without the process-level parts, so tests can drive it.
func run(args []string, stdout, stderr io.Writer, color bool) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch args[0] {
	case "check":
		return check(args[1:], stdout, stderr, color)
	case "analyze":
		return runAnalyze(args[1:], stdout, stderr, color)
	case "simulate":
		return runSimulate(args[1:], stdout, stderr)
	case "undo":
		return runUndo(args[1:], stdout, stderr)
	case "snapshots":
		return runSnapshots(stdout, stderr)
	case "guard":
		return runGuard(args[1:], stdout, stderr, color)
	case "hook":
		return runHook(args[1:], stdout, stderr)
	case "doctor":
		return doctor(stdout, probe())
	default:
		fmt.Fprintf(stderr, "ss: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func check(args []string, stdout, stderr io.Writer, color bool) int {
	asJSON := false
	if len(args) > 0 && args[0] == "--json" {
		asJSON, args = true, args[1:]
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	line := strings.Join(args, " ")
	v := classify.Classify(line)
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v)
	} else {
		printCheck(stdout, line, v, color)
	}
	switch v.Level {
	case domain.R4:
		return 4
	case domain.R3:
		return 3
	}
	return 0
}

var decision = [...]string{"ALLOW", "ALLOW (undo recorded)", "SIMULATE FIRST, THEN SNAPSHOT", "BLOCK (you can override)", "HARD BLOCK"}

func printCheck(w io.Writer, line string, v domain.Verdict, color bool) {
	c := newPalette(color)
	col := [...]string{c.green, c.green, c.yellow, c.red, c.red}[v.Level]
	fmt.Fprintf(w, "%s%s %s%s  %s\n", c.bold+col, v.Level, v.Level.Label(), c.reset, decision[v.Level])
	fmt.Fprintf(w, "%s$ %s%s\n", c.dim, line, c.reset)
	for _, f := range v.Findings {
		if f.Level == domain.R0 && v.Level > domain.R0 {
			continue
		}
		fmt.Fprintf(w, "\n  %s%s%s  %s\n", c.bold, f.Level, c.reset, f.Reason)
		if f.Command != line {
			fmt.Fprintf(w, "      %s%s%s\n", c.dim, f.Command, c.reset)
		}
	}
	if len(v.Alternatives) > 0 {
		fmt.Fprintf(w, "\n%sSafer options%s\n", c.bold, c.reset)
		for i, a := range v.Alternatives {
			fmt.Fprintf(w, "  %d) %s%s%s\n     %s\n", i+1, c.green, a.Command, c.reset, a.Note)
		}
	}
	if len(v.Unclassified) > 0 {
		fmt.Fprintf(w, "\n%sNot recognised yet: %s (rated R1; later phases test these in a sandbox)%s\n", c.dim, strings.Join(v.Unclassified, ", "), c.reset)
	}
}

type palette struct{ bold, dim, red, green, yellow, reset string }

func newPalette(on bool) palette {
	if !on {
		return palette{}
	}
	return palette{"\033[1m", "\033[2m", "\033[31m", "\033[32m", "\033[33m", "\033[0m"}
}

// useColor is true only when f is a terminal and NO_COLOR is unset.
func useColor(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0 && os.Getenv("NO_COLOR") == ""
}

func doctor(w io.Writer, r platform.Report) int {
	row := func(name string, ok bool, detail string) {
		mark := "ok     "
		if !ok {
			mark = "MISSING"
		}
		fmt.Fprintf(w, "  %s  %-26s %s\n", mark, name, detail)
	}
	fmt.Fprintf(w, "SafeShell doctor (kernel %s)\n\n", r.Kernel)
	row("user namespaces", r.UserNS, r.UserNSErr)
	row("overlay filesystem", r.Overlay, "")
	row("seccomp", r.Seccomp, "")
	row("landlock", r.Landlock, "")
	row("cgroup v2", r.CgroupV2, "")
	fmt.Fprintf(w, "  info     %-26s %s\n", "snapshot backend", r.SnapshotBackend()+" (root is "+r.RootFS+")")
	fmt.Fprintf(w, "  info     %-26s %s\n", "shells found", strings.Join(r.Shells, ", "))
	fmt.Fprintf(w, "\nSandbox simulation: %v\n", map[bool]string{true: "available", false: "NOT available"}[r.CanSimulate()])
	for _, h := range r.Hints() {
		fmt.Fprintf(w, "  - %s\n", h)
	}
	if r.CanSimulate() {
		return 0
	}
	return 1
}

func runSimulate(args []string, stdout, stderr io.Writer) int {
	spec := sandbox.Spec{}
	asJSON := false
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "--json":
			asJSON = true
		case "--cwd", "--timeout":
			if len(args) < 2 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			if args[0] == "--cwd" {
				spec.Cwd = args[1]
			} else if d, err := time.ParseDuration(args[1]); err != nil {
				fmt.Fprintf(stderr, "ss simulate: bad --timeout %q (try 5s)\n", args[1])
				return 2
			} else {
				spec.Timeout = d
			}
			args = args[1:]
		default:
			fmt.Fprintf(stderr, "ss simulate: unknown option %s\n", args[0])
			return 2
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	spec.Line = strings.Join(args, " ")
	res, err := simulate(context.Background(), spec)
	if err != nil {
		fmt.Fprintf(stderr, "ss simulate: %v\n", err)
		return 1
	}
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		simReport(stdout, spec.Line, res)
	}
	return 0
}

const maxListed = 15

func simReport(w io.Writer, line string, r sandbox.Result) {
	status := fmt.Sprintf("exit %d", r.ExitCode)
	if r.TimedOut {
		status = "timed out"
	}
	t := r.Diff.Totals()
	fmt.Fprintf(w, "Simulated in %s (%s). Your real files were not touched.\n$ %s\n\n", r.Duration.Round(time.Millisecond), status, line)
	fmt.Fprintf(w, "  created   %s\n  modified  %s\n  deleted   %s", report.Files(t.Created), report.Files(t.Modified), report.Files(t.Deleted))
	if t.Deleted > 0 {
		fmt.Fprintf(w, " (%s)", report.HumanBytes(t.DeletedBytes))
	}
	if r.Diff.Truncated {
		fmt.Fprint(w, " (count stopped at a limit)")
	}
	fmt.Fprintln(w)
	if len(r.Diff.Changes) > 0 {
		fmt.Fprintln(w, "\nChanges")
		for i, c := range r.Diff.Changes {
			if i == maxListed {
				fmt.Fprintf(w, "  ... and %d more\n", len(r.Diff.Changes)-maxListed)
				break
			}
			dir := ""
			if c.IsDir {
				dir = "/"
			}
			extra := ""
			if c.IsDir && c.Files > 0 {
				extra = fmt.Sprintf("  (%s)", report.Files(c.Files))
			}
			fmt.Fprintf(w, "  %-9s /%s%s%s\n", c.Kind, c.Path, dir, extra)
		}
	}
	for _, wn := range r.Warnings {
		fmt.Fprintf(w, "\nWarning: %s\n", wn)
	}
	for _, o := range []struct{ name, text string }{{"stdout", r.Stdout}, {"stderr", r.Stderr}} {
		if strings.TrimSpace(o.text) != "" {
			fmt.Fprintf(w, "\n%s\n%s", o.name, indent(o.text))
		}
	}
	if r.Truncated {
		fmt.Fprintln(w, "  (output cut)")
	}
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n") + "\n"
}
