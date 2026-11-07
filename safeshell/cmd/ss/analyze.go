package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"safeshell/internal/domain"
	"safeshell/internal/engine"
	"safeshell/internal/platform"
	"safeshell/internal/sandbox"
	"safeshell/internal/sandbox/effects"
	"safeshell/internal/sandbox/overlayfs"
	"safeshell/internal/shellparse"
)

// analyze is replaced in tests.
var analyze = func(ctx context.Context, line, cwd string) engine.Analysis {
	e := &engine.Engine{Sim: realSandbox{}, Inspectors: []engine.Inspector{inspectProcesses, inspectDisks}}
	return e.Analyze(ctx, line, cwd)
}

// realSandbox runs the OverlayFS sandbox, after checking the machine allows it.
type realSandbox struct{}

func (realSandbox) Simulate(ctx context.Context, spec sandbox.Spec) (sandbox.Result, error) {
	if r := platform.Probe(); !r.CanSimulate() {
		return sandbox.Result{}, fmt.Errorf("sandbox unavailable: %s", strings.Join(r.Hints(), " "))
	}
	return overlayfs.Run(ctx, spec)
}

func inspectProcesses(cmds []shellparse.Command) []domain.Effect {
	return effects.InspectProcesses(effects.ProcFS{Root: "/proc"}, os.Getpid(), os.Getuid(), cmds)
}

func inspectDisks(cmds []shellparse.Command) []domain.Effect {
	mi, _ := os.ReadFile("/proc/self/mountinfo")
	return effects.InspectDisks(effects.SysFS{Dev: "/dev", Sys: "/sys", Mountinfo: string(mi)}, cmds)
}

func runAnalyze(args []string, stdout, stderr io.Writer, color bool) int {
	cwd, asJSON := "", false
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		switch args[0] {
		case "--json":
			asJSON = true
		case "--cwd":
			if len(args) < 2 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			cwd, args = args[1], args[1:]
		default:
			fmt.Fprintf(stderr, "ss analyze: unknown option %s\n", args[0])
			return 2
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	line := strings.Join(args, " ")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	a := analyze(context.Background(), line, cwd)
	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(a)
	} else {
		printAnalysis(stdout, a, color)
	}
	switch a.Decision {
	case domain.HardBlock:
		return 4
	case domain.Block:
		return 3
	}
	return 0
}

var decisionText = map[domain.Decision]string{
	domain.Allow:     "ALLOW",
	domain.Confirm:   "CONFIRM (look at the impact first)",
	domain.Block:     "BLOCK (you can override)",
	domain.HardBlock: "HARD BLOCK",
}

func printAnalysis(w io.Writer, a engine.Analysis, color bool) {
	c := newPalette(color)
	col := [...]string{c.green, c.green, c.yellow, c.red, c.red}[a.Level]
	fmt.Fprintf(w, "%s%s %s%s  %s\n", c.bold+col, a.Level, a.Level.Label(), c.reset, decisionText[a.Decision])
	fmt.Fprintf(w, "%s$ %s%s\n", c.dim, a.Line, c.reset)
	if a.Level > a.Verdict.Level {
		fmt.Fprintf(w, "%sThe command text alone looked like %s; what it would really do raised it to %s.%s\n", c.dim, a.Verdict.Level, a.Level, c.reset)
	}

	shown := false
	for _, f := range a.Verdict.Findings {
		if f.Level == domain.R0 && a.Verdict.Level > domain.R0 {
			continue
		}
		if !shown {
			fmt.Fprintf(w, "\n%sWhy%s\n", c.bold, c.reset)
			shown = true
		}
		fmt.Fprintf(w, "  %s  %s\n", f.Level, f.Reason)
	}
	for _, e := range a.Effects {
		fmt.Fprintf(w, "\n%s%s%s  %s: %s\n", c.bold, e.Evidence, c.reset, e.Kind, e.Summary)
		for _, d := range e.Details {
			fmt.Fprintf(w, "      %s\n", d)
		}
	}
	if a.SimNote != "" {
		fmt.Fprintf(w, "\n%s%s%s\n", c.dim, a.SimNote, c.reset)
	}
	if a.Sim != nil {
		for _, wn := range a.Sim.Warnings {
			fmt.Fprintf(w, "%sSandbox note: %s%s\n", c.dim, wn, c.reset)
		}
	}
	if len(a.Verdict.Alternatives) > 0 {
		fmt.Fprintf(w, "\n%sSafer options%s\n", c.bold, c.reset)
		for i, alt := range a.Verdict.Alternatives {
			fmt.Fprintf(w, "  %d) %s%s%s\n     %s\n", i+1, c.green, alt.Command, c.reset, alt.Note)
		}
	}
}
