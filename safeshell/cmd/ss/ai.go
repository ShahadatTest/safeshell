package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"safeshell/internal/ai"
	"safeshell/internal/ai/cloud"
	"safeshell/internal/ai/local"
	"safeshell/internal/config"
	"safeshell/internal/domain"
	"safeshell/internal/engine"
	"safeshell/internal/nl"
)

// backend returns the configured AI, or nil when it is off. Tests replace it.
var backend = func() (ai.Backend, error) {
	c, err := config.Load(config.Path())
	if err != nil {
		return nil, err
	}
	var chain ai.Chain
	if c.AI.Mode == ai.Local || c.AI.Mode == ai.Hybrid {
		chain = append(chain, local.Ollama{URL: c.AI.Local.URL, Model: c.AI.Local.Model})
	}
	if c.AI.Mode == ai.Cloud || c.AI.Mode == ai.Hybrid {
		chain = append(chain, ai.Redacting(cloud.Anthropic{Key: os.Getenv("ANTHROPIC_API_KEY"), Model: c.AI.Cloud.Model}))
	}
	if len(chain) == 0 {
		return nil, nil
	}
	return chain, nil
}

// aiReview lets the model read the sandbox result and raise the risk. Any
// failure is shown but never blocks the command.
func aiReview(a *engine.Analysis, stderr io.Writer) {
	if a.Level == domain.R0 || a.Level >= domain.R4 {
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
		fmt.Fprintln(stderr, "ss explain: AI is off. Set ai.mode to local, cloud or hybrid in", config.Path(), "(or SAFESHELL_AI=local).")
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

// runAsk turns English into a command, then treats it exactly like a typed
// one: analysis, prompt, snapshot. The chosen command is printed.
func runAsk(args []string, stdout, stderr io.Writer, color bool) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	b, err := backend()
	if err != nil || b == nil {
		fmt.Fprintln(stderr, "ss ask: AI is off. Set ai.mode to local, cloud or hybrid in", config.Path(), "(or SAFESHELL_AI=local).")
		return 1
	}
	cwd, _ := os.Getwd()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r, err := nl.Translate(ctx, b, strings.Join(args, " "), cwd)
	if err != nil {
		fmt.Fprintln(stderr, "ss ask:", firstErrLine(err))
		return 1
	}
	fmt.Fprintf(stderr, "Proposed: %s\n", r.Command)
	if r.Note != "" {
		fmt.Fprintf(stderr, "          %s\n", r.Note)
	}
	return guardLine(r.Command, cwd, stdout, stderr, color)
}
