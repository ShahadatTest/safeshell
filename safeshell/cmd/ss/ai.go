package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"safeshell/internal/ai"
	"safeshell/internal/domain"
	"safeshell/internal/engine"
	"safeshell/internal/nl"
)

// aiReview lets the model read the sandbox result and raise the risk. Any
// failure is shown but never blocks the command.
func aiReview(a *engine.Analysis, stderr io.Writer) {
	// Only commands that already need a decision are worth a model call: it can
	// take a second or more, and it can only raise a level that matters.
	if a.Level < domain.R2 || a.Level >= domain.R4 {
		return
	}
	b, err := backend()
	if err != nil {
		fmt.Fprintln(stderr, "AI review skipped:", err)
		return
	}
	if b == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	v, raised, err := ai.Review(ctx, b, *a)
	switch {
	case err != nil:
		fmt.Fprintln(stderr, "AI review unavailable:", firstErrLine(err))
	case raised:
		a.AddEffect(ai.Effect(b, v))
	}
}

func firstErrLine(err error) string {
	s, _, _ := strings.Cut(err.Error(), "\n")
	return s
}

// runExplain prints the analysis and a plain-English explanation.
func runExplain(args []string, stdout, stderr io.Writer, color bool) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	b, err := backend()
	if err != nil || b == nil {
		fmt.Fprintln(stderr, "ss explain: AI is off. Type /models to choose a model, or run  ss ai setup")
		return 1
	}
	cwd, _ := os.Getwd()
	a := analyze(context.Background(), strings.Join(args, " "), cwd)
	printAnalysis(stdout, a, color)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	text, err := ai.Explain(ctx, b, a)
	if err != nil {
		fmt.Fprintln(stderr, "ss explain:", firstErrLine(err))
		return 1
	}
	fmt.Fprintf(stdout, "\nIn plain English\n  %s\n", text)
	return 0
}

// runAsk is `ss ask <english>`: translate, then treat the result like a typed command.
func runAsk(args []string, stdout, stderr io.Writer, color bool) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cwd, _ := os.Getwd()
	code, _ := askLine(strings.Join(args, " "), cwd, stdout, stderr, color)
	return code
}

// askLine turns a plain-English request into a command and runs it through
// the same guard as a typed one. The second result is false when no AI was
// available, so a caller that guessed this was English can let the line through.
func askLine(request, cwd string, stdout, stderr io.Writer, color bool) (code int, available bool) {
	b, err := backend()
	if err != nil || b == nil {
		fmt.Fprintln(stderr, "SafeShell AI is off. Type /models to choose a model, or run  ss ai setup")
		return 1, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	r, err := nl.Translate(ctx, b, request, gatherEnv(cwd))
	if err != nil {
		fmt.Fprintln(stderr, "ss ask:", firstErrLine(err))
		return 1, !strings.Contains(err.Error(), "ss ai setup") && !strings.Contains(err.Error(), "local engine")
	}
	if r.Command == "" { // a question: show the answer, run nothing
		fmt.Fprintln(stderr, r.Answer)
		return 1, true
	}
	fmt.Fprintf(stderr, "Proposed: %s\n", r.Command)
	if r.Note != "" {
		fmt.Fprintf(stderr, "          %s\n", r.Note)
	}
	return guardLine(r.Command, cwd, stdout, stderr, color, true), true
}
