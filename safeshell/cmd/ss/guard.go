package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"safeshell/internal/audit"
	"safeshell/internal/domain"
	"safeshell/internal/engine"
	"safeshell/internal/shellhook"
	"safeshell/internal/ui"
)

// guardInput is where answers are read from; tests replace it.
var guardInput = func() io.Reader {
	if t, err := os.Open("/dev/tty"); err == nil {
		return t
	}
	return os.Stdin
}

// runGuard is what the shell hook calls for every command line. The command to
// run goes to stdout (the original, or a safer option the person picked); the
// analysis and questions go to stderr. Exit 0 runs it, 1 cancels.
func runGuard(args []string, stdout, stderr io.Writer, color bool) int {
	cwd := ""
opts:
	for len(args) > 0 {
		switch {
		case args[0] == "--":
			args = args[1:]
			break opts
		case args[0] == "--cwd" && len(args) >= 2:
			cwd, args = args[1], args[2:]
		case strings.HasPrefix(args[0], "-"):
			fmt.Fprintf(stderr, "ss guard: bad option %s\n", args[0])
			return 2
		default:
			break opts
		}
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
	if a.Decision != domain.Allow {
		printAnalysis(stderr, a, color)
	}
	cmd, ok := ui.Choose(bufio.NewReader(guardInput()), stderr, line, a.Decision, a.Verdict.Alternatives)
	record(a, cmd, ok)
	if !ok {
		fmt.Fprintln(stderr, "Cancelled. Nothing was run.")
		return 1
	}
	fmt.Fprintln(stdout, cmd)
	return 0
}

func runHook(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	s, err := shellhook.Script(args[0])
	if err != nil {
		fmt.Fprintln(stderr, "ss hook:", err)
		return 2
	}
	fmt.Fprint(stdout, s)
	return 0
}

// auditPath is where decisions are logged; tests replace it.
var auditPath = audit.DefaultPath

func record(a engine.Analysis, cmd string, ran bool) {
	e := audit.Entry{Time: time.Now().UTC(), Line: a.Line, Level: a.Level.String(), Decision: a.Decision.String(), Outcome: "cancelled"}
	switch {
	case ran && a.Decision == domain.Allow:
		e.Outcome = "allowed"
	case ran && cmd != a.Line:
		e.Outcome, e.Run = "replaced", cmd
	case ran:
		e.Outcome = "ran"
	}
	_ = audit.Append(auditPath(), e) // a full disk must not stop a command
}
