package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"safeshell/internal/install"
)

func shellFor() (install.Shell, error) {
	home, _ := os.UserHomeDir()
	return install.Detect(os.Getenv("SHELL"), home)
}

// runInstall is `ss install [--ai]`: it adds the hook to your shell startup file.
func runInstall(args []string, stdout, stderr io.Writer) int {
	withAI := false
	for _, a := range args {
		if a != "--ai" {
			fmt.Fprint(stderr, usage)
			return 2
		}
		withAI = true
	}
	sh, err := shellFor()
	if err != nil {
		fmt.Fprintln(stderr, "ss install:", err, "\nYou can still add the hook by hand: ss hook bash|zsh")
		return 1
	}
	bin, err := os.Executable()
	if err == nil {
		bin, err = filepath.EvalSymlinks(bin)
	}
	if err != nil {
		fmt.Fprintln(stderr, "ss install:", err)
		return 1
	}
	changed, err := install.Add(sh, bin)
	if err != nil {
		fmt.Fprintln(stderr, "ss install:", err)
		return 1
	}
	if changed {
		fmt.Fprintf(stdout, "Added the SafeShell hook to %s (a copy of the old file is %s.safeshell-backup).\n", sh.RC, sh.RC)
	} else {
		fmt.Fprintf(stdout, "The SafeShell hook is already in %s.\n", sh.RC)
	}
	fmt.Fprintln(stdout, "Open a new terminal (or run:  source "+sh.RC+" ) and every command you press Enter on is checked first.")
	if r := probe(); !r.CanSimulate() {
		fmt.Fprintln(stdout, "\nThe sandbox is not available on this machine yet:")
		for _, h := range r.Hints() {
			fmt.Fprintln(stdout, "  -", h)
		}
		fmt.Fprintln(stdout, "  Fix on Ubuntu 24.04+:  scripts/install-apparmor.sh", bin)
	}
	if withAI {
		fmt.Fprintln(stdout)
		return runAI([]string{"setup"}, stdout, stderr)
	}
	fmt.Fprintln(stdout, "\nNext: ss ai setup   (downloads the built-in AI, optional)   |   ss help start")
	return 0
}

// runUninstall is `ss uninstall`: it removes the hook and leaves everything else alone.
func runUninstall(stdout, stderr io.Writer) int {
	sh, err := shellFor()
	if err != nil {
		fmt.Fprintln(stderr, "ss uninstall:", err)
		return 1
	}
	changed, err := install.Remove(sh)
	if err != nil {
		fmt.Fprintln(stderr, "ss uninstall:", err)
		return 1
	}
	if !changed {
		fmt.Fprintf(stdout, "The SafeShell hook was not in %s.\n", sh.RC)
		return 0
	}
	fmt.Fprintf(stdout, "Removed the SafeShell hook from %s. New terminals are no longer guarded.\n", sh.RC)
	if !strings.Contains(os.Getenv("SAFESHELL_OFF"), "1") {
		fmt.Fprintln(stdout, "This terminal still has it until you close it. Your snapshots and settings were kept.")
	}
	return 0
}
