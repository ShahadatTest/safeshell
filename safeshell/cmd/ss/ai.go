package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	if c.AI.Mode == ai.Auto {
		c.AI.Mode = ai.Off
		if _, err := exec.LookPath("ollama"); err == nil {
			c.AI.Mode = ai.Local
		}
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
		fmt.Fprintln(stderr, "SafeShell AI is off or not installed. Run:  ss ai setup")
		return 1, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	r, err := nl.Translate(ctx, b, request, gatherEnv(cwd))
	if err != nil {
		fmt.Fprintln(stderr, "ss ask:", firstErrLine(err))
		return 1, !strings.Contains(err.Error(), "ss ai setup") && !strings.Contains(err.Error(), "Ollama")
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

// runAI is `ss ai setup|status`.
func runAI(args []string, stdout, stderr io.Writer) int {
	c, err := config.Load(config.Path())
	if err != nil {
		fmt.Fprintln(stderr, "ss ai:", err)
		return 1
	}
	o := local.Ollama{URL: c.AI.Local.URL, Model: c.AI.Local.Model}
	ctx := context.Background()
	switch {
	case len(args) == 1 && args[0] == "status":
		b, _ := backend()
		if b == nil {
			fmt.Fprintf(stdout, "AI: off (mode %s). Run: ss ai setup\n", c.AI.Mode)
			return 0
		}
		fmt.Fprintf(stdout, "AI: %s (mode %s)\n", b.Name(), c.AI.Mode)
		if have, err := o.Models(ctx); err != nil {
			fmt.Fprintln(stdout, "Local model server: not running (it starts automatically when needed)")
		} else if local.HasModel(have, o.Model) {
			fmt.Fprintf(stdout, "Local model %s: downloaded\n", o.Model)
		} else {
			fmt.Fprintf(stdout, "Local model %s: not downloaded. Run: ss ai setup\n", o.Model)
		}
		return 0
	case len(args) == 1 && args[0] == "setup":
		if err := o.EnsureServer(ctx); err != nil {
			fmt.Fprintln(stderr, "ss ai setup:", err)
			return 1
		}
		have, _ := o.Models(ctx)
		if local.HasModel(have, o.Model) {
			fmt.Fprintf(stdout, "Ready: %s is already downloaded. Try:  ss ask \"show the biggest files here\"\n", o.Model)
			return 0
		}
		fmt.Fprintf(stdout, "Downloading %s (a one-time download of about 2 GB)...\n", o.Model)
		if err := o.Pull(ctx, stdout); err != nil {
			fmt.Fprintln(stderr, "ss ai setup:", err)
			return 1
		}
		fmt.Fprintln(stdout, "Ready. In a shell with the hook, just type what you want, or start a line with ?")
		return 0
	}
	fmt.Fprint(stderr, usage)
	return 2
}
