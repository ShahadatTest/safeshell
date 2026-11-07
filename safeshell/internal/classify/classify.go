package risk

import (
	"regexp"
	"slices"
	"strings"

	"safeshell/internal/shellparse"
)

// Classify rates a whole command line. The result is the highest level of any
// command in it; every reason and safer alternative is kept for the report.
func Classify(line string) Verdict {
	if strings.TrimSpace(line) == "" {
		return Verdict{Level: R0}
	}
	res, err := shellparse.Parse(line)
	if err != nil {
		return Verdict{Level: R2, Findings: []Finding{{
			Level:   R2,
			Command: line,
			Reason:  "Could not be understood as a shell command, so it cannot be checked. Review it by eye.",
		}}}
	}
	var fs []Finding
	var unclassified []string
	if res.ForkBomb {
		fs = append(fs, Finding{Level: R4, Command: line, Reason: "A fork bomb: it starts copies of itself until the machine runs out of resources and freezes."})
	}
	for _, p := range res.Pipelines {
		fs = append(fs, pipelineFindings(p)...)
		for _, c := range p.Cmds {
			cf, unknown := commandFindings(c)
			fs = append(fs, cf...)
			if unknown {
				unclassified = append(unclassified, c.Name)
			}
		}
	}
	v := Verdict{Findings: fs, Unclassified: dedupe(unclassified)}
	v.Level = maxLevel(fs)
	seen := map[string]bool{}
	for _, f := range fs {
		if f.Alt != nil && f.Level >= R2 && !seen[f.Alt.Command] {
			seen[f.Alt.Command] = true
			v.Alternatives = append(v.Alternatives, *f.Alt)
		}
	}
	return v
}

func maxLevel(fs []Finding) Level {
	m := R0
	for _, f := range fs {
		m = max(m, f.Level)
	}
	return m
}

func dedupe(ss []string) []string {
	slices.Sort(ss)
	return slices.Compact(ss)
}

func matches(pattern, s string) bool { return regexp.MustCompile(pattern).MatchString(s) }

// commandFindings applies the rule for one command, then adds redirect and
// privilege effects. The bool is true when the command is not in the rule table.
func commandFindings(c shellparse.Command) ([]Finding, bool) {
	var fs []Finding
	unknown := false
	if c.Name != "" {
		f := parseFlags(c.Args)
		if r, ok := rules[c.Name]; ok {
			fs = r(c, f)
		} else if slices.Contains(readOnly, c.Name) {
			fs = one(fnd(R0, c, "Only reads information."))
		} else if slices.Contains(devTools, c.Name) {
			fs = one(fnd(R1, c, "Runs a program or edits files; effects depend on what it is given."))
		} else {
			unknown = true
			fs = one(fnd(R1, c, "Not a command SafeShell recognises yet. It will be tested in the sandbox before it runs."))
		}
	}
	fs = append(fs, redirectFindings(c)...)
	if c.Sudo {
		for i := range fs {
			if fs[i].Level >= R1 {
				fs[i].Level = min(R4, fs[i].Level+1)
				fs[i].Reason += " It runs as root, so system files are not protected."
			}
		}
	}
	if c.Remote != "" {
		for i := range fs {
			fs[i].Reason = "On remote host " + c.Remote + ": " + fs[i].Reason
		}
	}
	return fs, unknown
}

var harmlessTargets = map[string]bool{"/dev/null": true, "/dev/stdout": true, "/dev/stderr": true, "/dev/tty": true, "/dev/zero": true}

func redirectFindings(c shellparse.Command) []Finding {
	var fs []Finding
	for _, r := range c.Redirs {
		if r.Op != ">" && r.Op != ">>" && r.Op != ">|" && r.Op != "&>" && r.Op != "&>>" && r.Op != "<>" {
			continue
		}
		t := normPath(r.Target)
		appending := strings.HasSuffix(r.Op, ">>")
		cmd := c.String()
		if cmd == "" {
			cmd = r.Op + " " + r.Target
		}
		switch {
		case harmlessTargets[t] || strings.HasPrefix(t, "/dev/fd/"):
		case isBlockDevice(t) || t == "/dev/mem" || t == "/dev/kmem":
			fs = append(fs, Finding{R4, cmd, "Writes raw data onto " + t + ", destroying what is stored there.", nil})
		case strings.HasPrefix(t, "/proc/") || strings.HasPrefix(t, "/sys/"):
			fs = append(fs, Finding{R4, cmd, "Writes to a live kernel control file (" + t + "), which can crash or reboot the machine.", nil})
		case underSystem(t) && !appending:
			fs = append(fs, Finding{R3, cmd, "Overwrites the system file " + t + ".", &Alternative{Command: "cp " + shq(t) + " " + shq(t) + ".bak", Note: "make a backup of the file first"}})
		case underSystem(t):
			fs = append(fs, Finding{R2, cmd, "Appends to the system file " + t + ".", nil})
		case appending:
			fs = append(fs, Finding{R1, cmd, "Appends to " + t + ".", nil})
		default:
			fs = append(fs, Finding{R2, cmd, "Overwrites " + t + " if it already exists.", &Alternative{Command: strings.Replace(cmd, r.Op+" "+r.Target, ">> "+r.Target, 1), Note: "appends instead of overwriting"}})
		}
	}
	return fs
}

var fetchers = map[string]bool{"curl": true, "wget": true, "fetch": true, "base64": true, "xxd": true, "openssl": true, "nc": true, "ncat": true}

// pipelineFindings looks for dangerous patterns that only exist across a pipe,
// such as downloading code and running it unseen.
func pipelineFindings(p shellparse.Pipeline) []Finding {
	var fs []Finding
	for i, c := range p.Cmds {
		if !interpreters[c.Name] || !readsStdinOrDownload(c) {
			continue
		}
		fromNet := c.Downloads || (i > 0 && fetchers[p.Cmds[i-1].Name])
		if !fromNet {
			continue
		}
		lvl := R3
		if c.Sudo {
			lvl = R4
		}
		fs = append(fs, Finding{lvl, c.String(), "Downloads code and runs it immediately without you reading it. Whoever controls that server controls your machine.",
			&Alternative{Command: "curl -fsSL URL -o script.sh && less script.sh && bash script.sh", Note: "download first, read it, then run it"}})
	}
	return fs
}

func readsStdinOrDownload(c shellparse.Command) bool {
	if c.Downloads {
		return true
	}
	f := parseFlags(c.Args)
	return len(f.pos) == 0 || (len(f.pos) == 1 && f.pos[0] == "-")
}
