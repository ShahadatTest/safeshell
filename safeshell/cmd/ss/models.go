package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"safeshell/eval"
	"safeshell/internal/ai"
	"safeshell/internal/ai/providers"
	"safeshell/internal/config"
	"safeshell/internal/models"
	"safeshell/internal/report"
	"safeshell/internal/runtime"
)

// modelMenu is the /models screen. Its effects are injected so tests do not
// download gigabytes or call paid APIs.
type modelMenu struct {
	in      *bufio.Reader
	out     io.Writer
	cfgPath string
	dirs    runtime.Dirs
	install func(ctx context.Context, cfg config.Config, m models.Model, w io.Writer) error
	check   func(p providers.Provider, key, model string) error
	secret  func(prompt string) (string, error)
}

func newModelMenu(in io.Reader, out io.Writer) *modelMenu {
	return &modelMenu{
		in: bufio.NewReader(in), out: out, cfgPath: config.Path(), dirs: runtime.DefaultDirs(),
		install: installLocal,
		check:   func(p providers.Provider, key, model string) error { return providers.Check(p.Backend(key, model)) },
		secret:  readSecret,
	}
}

// installLocal downloads the engine for this machine and the model.
func installLocal(ctx context.Context, cfg config.Config, m models.Model, w io.Writer) error {
	d := runtime.DefaultDirs()
	b, ok := runtime.Pick(goruntime.GOARCH, cfg.AI.Local.GPU)
	if !ok {
		return fmt.Errorf("there is no local engine for %s; use a cloud model instead", goruntime.GOARCH)
	}
	if err := d.InstallEngine(ctx, nil, b, w); err != nil {
		return err
	}
	return d.InstallModel(ctx, nil, m, w)
}

func readSecret(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", errors.New("no terminal to read the key from; set the key in your environment instead")
	}
	defer tty.Close()
	fmt.Fprint(tty, prompt)
	b, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty)
	return strings.TrimSpace(string(b)), err
}

func (m *modelMenu) line(prompt string) (string, bool) {
	fmt.Fprint(m.out, prompt)
	s, err := m.in.ReadString('\n')
	return strings.TrimSpace(s), err == nil || s != ""
}

func (m *modelMenu) render(c config.Config) {
	fmt.Fprintf(m.out, "\nSafeShell AI models        mode: %s\n", c.AI.Mode)
	fmt.Fprintln(m.out, "\n  On this machine (private, free, works offline)")
	for i, md := range models.Catalog {
		state := "not downloaded"
		if m.dirs.ModelInstalled(md) {
			state = "downloaded"
		}
		mark := ""
		if md.ID == c.AI.Local.Model && (c.AI.Mode == ai.Local || c.AI.Mode == ai.Auto || c.AI.Mode == ai.Hybrid) {
			mark = "  <- in use"
		}
		fmt.Fprintf(m.out, "  %d) %-20s %6s  %-14s %s%s\n", i+1, md.Title, report.HumanBytes(md.Size), state, md.Note, mark)
	}
	fmt.Fprintln(m.out, "\n  In the cloud (needs your API key; the command text is sent, your home folder name hidden)")
	for i, p := range providers.All {
		key := "no key yet"
		if config.Key(p) != "" {
			key = "key saved"
		}
		model := p.DefaultModel
		if c.AI.Cloud.Provider == p.ID && c.AI.Cloud.Model != "" {
			model = c.AI.Cloud.Model
		}
		mark := ""
		if c.AI.Cloud.Provider == p.ID && (c.AI.Mode == ai.Cloud || c.AI.Mode == ai.Hybrid) {
			mark = "  <- in use"
		}
		fmt.Fprintf(m.out, "  %d) %-7s %-10s %-18s %s%s\n", len(models.Catalog)+i+1, p.Title, "("+p.Maker+")", model, key, mark)
	}
	fmt.Fprintln(m.out, "\n  A number picks (and downloads or connects) it.   key N: change a key   model N NAME: change a cloud model")
	fmt.Fprintln(m.out, "  off / auto / local / cloud / hybrid: change the mode.   q: leave")
}

// Run shows the menu until the person leaves.
func (m *modelMenu) Run(ctx context.Context) {
	for {
		c, err := config.Load(m.cfgPath)
		if err != nil {
			fmt.Fprintln(m.out, "Cannot read the settings:", err)
			return
		}
		m.render(c)
		in, ok := m.line("\n> ")
		if !ok || in == "" || in == "q" || in == "quit" {
			return
		}
		m.handle(ctx, c, strings.Fields(in))
	}
}

func (m *modelMenu) handle(ctx context.Context, c config.Config, f []string) {
	switch f[0] {
	case "off", "auto", "local", "cloud", "hybrid":
		c.AI.Mode = ai.Mode(f[0])
		m.save(c, "Mode is now "+f[0]+".")
		return
	case "key":
		if p, ok := m.cloudAt(f, 1); ok {
			m.connect(ctx, c, p, true)
		}
		return
	case "model":
		if p, ok := m.cloudAt(f, 1); ok && len(f) == 3 {
			c.AI.Cloud.Provider, c.AI.Cloud.Model = p.ID, f[2]
			m.save(c, p.Title+" will use the model "+f[2]+".")
		} else if ok {
			fmt.Fprintln(m.out, "Usage: model N NAME   (for example: model 6 gpt-4.1)")
		}
		return
	}
	n, err := strconv.Atoi(f[0])
	switch {
	case err != nil || n < 1 || n > len(models.Catalog)+len(providers.All):
		fmt.Fprintln(m.out, "Please type one of the numbers, a mode, or q.")
	case n <= len(models.Catalog):
		m.useLocal(ctx, c, models.Catalog[n-1])
	default:
		m.connect(ctx, c, providers.All[n-len(models.Catalog)-1], false)
	}
}

func (m *modelMenu) cloudAt(f []string, i int) (providers.Provider, bool) {
	n, err := strconv.Atoi(strings.Join(f[min(i, len(f)-1):min(i+1, len(f))], ""))
	k := n - len(models.Catalog) - 1
	if err != nil || k < 0 || k >= len(providers.All) {
		fmt.Fprintf(m.out, "That needs a cloud model number (%d to %d).\n", len(models.Catalog)+1, len(models.Catalog)+len(providers.All))
		return providers.Provider{}, false
	}
	return providers.All[k], true
}

func (m *modelMenu) useLocal(ctx context.Context, c config.Config, md models.Model) {
	if !m.dirs.ModelInstalled(md) {
		a, _ := m.line(fmt.Sprintf("Download %s (%s)? [Y/n] ", md.Title, report.HumanBytes(md.Size)))
		if a != "" && !strings.EqualFold(a, "y") && !strings.EqualFold(a, "yes") {
			return
		}
		if err := m.install(ctx, c, md, m.out); err != nil {
			fmt.Fprintln(m.out, "Download failed:", err)
			return
		}
	}
	c.AI.Local.Model = md.ID
	if c.AI.Mode != ai.Hybrid {
		c.AI.Mode = ai.Local
	}
	m.save(c, md.Title+" is ready and in use.")
}

func (m *modelMenu) connect(ctx context.Context, c config.Config, p providers.Provider, replace bool) {
	key := config.Key(p)
	if key == "" || replace {
		fmt.Fprintf(m.out, "Create a key at %s\n", p.KeyURL)
		k, err := m.secret("Paste your " + p.Title + " API key (hidden): ")
		if err != nil || len(k) < 10 || strings.ContainsAny(k, " \t") {
			fmt.Fprintln(m.out, "That does not look like an API key; nothing was saved.")
			return
		}
		fmt.Fprintln(m.out, "Checking the key...")
		if err := m.check(p, k, c.AI.Cloud.Model); err != nil {
			fmt.Fprintln(m.out, "The key did not work:", err)
			return
		}
		if err := config.SaveKey(p, k); err != nil {
			fmt.Fprintln(m.out, "Could not save the key:", err)
			return
		}
	}
	if c.AI.Cloud.Provider != p.ID {
		c.AI.Cloud.Model = ""
	}
	c.AI.Cloud.Provider = p.ID
	if c.AI.Mode != ai.Hybrid {
		c.AI.Mode = ai.Cloud
	}
	m.save(c, p.Title+" ("+p.Maker+") is connected and in use.")
}

func (m *modelMenu) save(c config.Config, msg string) {
	if err := config.Save(m.cfgPath, c); err != nil {
		fmt.Fprintln(m.out, "Could not save the settings:", err)
		return
	}
	fmt.Fprintln(m.out, msg)
}

// runModels is `ss models` and the /models command.
func runModels(in io.Reader, out io.Writer) int {
	newModelMenu(in, out).Run(context.Background())
	return 0
}

// runAI is `ss ai setup|status|stop`.
func runAI(args []string, stdout, stderr io.Writer) int {
	c, err := config.Load(config.Path())
	if err != nil {
		fmt.Fprintln(stderr, "ss ai:", err)
		return 1
	}
	d := runtime.DefaultDirs()
	md, _ := models.Find(c.AI.Local.Model)
	switch {
	case len(args) == 1 && args[0] == "setup":
		if err := installLocal(context.Background(), c, md, stdout); err != nil {
			fmt.Fprintln(stderr, "ss ai setup:", err)
			return 1
		}
		fmt.Fprintf(stdout, "Ready: %s. Type English in your terminal, start a line with ?, or run  ss ask \"show the biggest files here\".\nOther models, or Claude / GPT / Gemini: type /models\n", md.Title)
		return 0
	case len(args) == 1 && args[0] == "stop":
		if d.Stop() {
			fmt.Fprintln(stdout, "The local AI engine was stopped. It starts again when needed.")
		} else {
			fmt.Fprintln(stdout, "The local AI engine is not running.")
		}
		return 0
	case len(args) == 1 && args[0] == "status":
		b, _ := backend()
		if b == nil {
			fmt.Fprintf(stdout, "AI: off (mode %s). Type /models to choose a model.\n", c.AI.Mode)
		} else {
			fmt.Fprintf(stdout, "AI: %s (mode %s)\n", b.Name(), c.AI.Mode)
		}
		state := "not downloaded"
		if d.ModelInstalled(md) {
			state = "downloaded"
		}
		fmt.Fprintf(stdout, "Local model %s: %s\n", md.Title, state)
		if run, ok := d.Running(); ok {
			fmt.Fprintf(stdout, "Local engine: running (%s)\n", run)
		} else {
			fmt.Fprintln(stdout, "Local engine: not running (starts when needed, stops itself after 10 idle minutes)")
		}
		return 0
	}
	if len(args) >= 1 && args[0] == "bench" {
		return runBench(c, args[1:], stdout, stderr)
	}
	fmt.Fprint(stderr, usage)
	return 2
}

// runBench measures how well the active model (or --model ID) turns English
// into safe, correct commands, using the labelled set in eval/requests.tsv.
func runBench(c config.Config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 2 && args[0] == "--model" {
		if _, ok := models.Find(args[1]); !ok {
			fmt.Fprintln(stderr, "ss ai bench: unknown model", args[1])
			return 2
		}
		c.AI.Mode, c.AI.Local.Model = ai.Local, args[1]
	} else if len(args) != 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	b, err := buildBackend(c, runtime.DefaultDirs())
	if err != nil || b == nil {
		fmt.Fprintln(stderr, "ss ai bench: no AI is active. Type /models to choose one.")
		return 1
	}
	reqs, err := eval.LoadRequests()
	if err != nil {
		fmt.Fprintln(stderr, "ss ai bench:", err)
		return 1
	}
	cwd, _ := os.Getwd()
	fmt.Fprintf(stdout, "Testing %s on %d requests...\n", b.Name(), len(reqs))
	res := eval.RunBench(context.Background(), b, gatherEnv(cwd), reqs, func(o eval.Outcome) {
		mark := "ok  "
		if !o.Pass {
			mark = "FAIL"
		}
		got := o.Command
		if got == "" {
			got = o.Answer
		}
		fmt.Fprintf(stdout, "%s %5.1fs  %s\n        -> %s\n", mark, o.Elapsed.Seconds(), o.Request.Text, got)
		if !o.Pass {
			fmt.Fprintf(stdout, "        why: %s\n", o.Why)
		}
	})
	var total time.Duration
	for _, o := range res.Outcomes {
		total += o.Elapsed
	}
	n := len(res.Outcomes)
	fmt.Fprintf(stdout, "\n%s: %d of %d correct (%d%%), %d too dangerous, %.1fs per request on average\n",
		res.Backend, res.Passed(), n, res.Passed()*100/max(n, 1), res.Unsafe(), total.Seconds()/float64(max(n, 1)))
	return 0
}
