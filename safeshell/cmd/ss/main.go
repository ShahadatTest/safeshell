// Command ss is the SafeShell command-line tool.
//
// Phase 1 provides `ss check`, which rates a command without running it.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"safeshell/internal/classify"
	"safeshell/internal/domain"
)

const usage = `SafeShell: a safety layer for your terminal.

Usage:
  ss check [--json] <command...>   rate a command without running it

Quote the command so your shell passes it through untouched:
  ss check 'rm -rf ~/projects'

Exit status of 'ss check': 0 = R0-R2, 3 = R3 (irreversible), 4 = R4 (catastrophic).
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(usage)
		return
	}
	switch args[0] {
	case "check":
		os.Exit(check(args[1:]))
	default:
		fmt.Fprintf(os.Stderr, "ss: unknown command %q\n\n%s", args[0], usage)
		os.Exit(2)
	}
}

func check(args []string) int {
	asJSON := false
	if len(args) > 0 && args[0] == "--json" {
		asJSON, args = true, args[1:]
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	line := strings.Join(args, " ")
	v := classify.Classify(line)
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v)
	} else {
		report(line, v)
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

func report(line string, v domain.Verdict) {
	c := newPalette(os.Stdout)
	col := [...]string{c.green, c.green, c.yellow, c.red, c.red}[v.Level]
	fmt.Printf("%s%s %s%s  %s\n", c.bold+col, v.Level, v.Level.Label(), c.reset, decision[v.Level])
	fmt.Printf("%s$ %s%s\n", c.dim, line, c.reset)
	for _, f := range v.Findings {
		if f.Level == domain.R0 && v.Level > domain.R0 {
			continue
		}
		fmt.Printf("\n  %s%s%s  %s\n", c.bold, f.Level, c.reset, f.Reason)
		if f.Command != line {
			fmt.Printf("      %s%s%s\n", c.dim, f.Command, c.reset)
		}
	}
	if len(v.Alternatives) > 0 {
		fmt.Printf("\n%sSafer options%s\n", c.bold, c.reset)
		for i, a := range v.Alternatives {
			fmt.Printf("  %d) %s%s%s\n     %s\n", i+1, c.green, a.Command, c.reset, a.Note)
		}
	}
	if len(v.Unclassified) > 0 {
		fmt.Printf("\n%sNot recognised yet: %s (rated R1; later phases test these in a sandbox)%s\n", c.dim, strings.Join(v.Unclassified, ", "), c.reset)
	}
}

type palette struct{ bold, dim, red, green, yellow, reset string }

// newPalette returns ANSI colours only when writing to a terminal and NO_COLOR is unset.
func newPalette(f *os.File) palette {
	st, err := f.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 || os.Getenv("NO_COLOR") != "" {
		return palette{}
	}
	return palette{"\033[1m", "\033[2m", "\033[31m", "\033[32m", "\033[33m", "\033[0m"}
}
