package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"safeshell/internal/nl"
)

// usefulTools are the programs worth telling the model about when installed.
var usefulTools = strings.Fields(`git rg fd fzf jq yq curl wget tar zip unzip rsync ssh docker podman kubectl systemctl journalctl
apt dnf pacman snap pip npm node go cargo python3 make gcc ffmpeg convert trash-put tree ncdu htop lsof ss ip`)

// gatherEnv describes this machine and directory for the assistant.
func gatherEnv(cwd string) nl.Env {
	e := nl.Env{OS: osName(), Shell: filepath.Base(os.Getenv("SHELL")), Cwd: cwd}
	if des, err := os.ReadDir(cwd); err == nil {
		for _, de := range des {
			n := de.Name()
			if de.IsDir() {
				n += "/"
			}
			e.Listing = append(e.Listing, n)
		}
		sort.Strings(e.Listing)
	}
	for _, t := range usefulTools {
		if _, err := exec.LookPath(t); err == nil {
			e.Tools = append(e.Tools, t)
		}
	}
	return e
}

func osName() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return "Linux"
}
