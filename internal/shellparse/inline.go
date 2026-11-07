package shellparse

import (
	"regexp"
	"strings"
)

// inlineFlags are the options that pass program text on the command line.
var inlineFlags = map[string][]string{
	"python": {"-c"}, "python3": {"-c"}, "perl": {"-e", "-E"}, "ruby": {"-e"}, "node": {"-e", "-p"},
}

// InlineCode returns the program text given with -c / -e, or "" if there is none.
func InlineCode(c Command) string {
	flags := inlineFlags[c.Name]
	for i, a := range c.Args {
		for _, f := range flags {
			if a == f && i+1 < len(c.Args) {
				return c.Args[i+1]
			}
		}
	}
	return ""
}

var (
	// os.system("..."), subprocess.run("..."), system("..."), execSync("...").
	// RE2 has no backreferences, so each quote style is its own alternative.
	callString = regexp.MustCompile(`(?:os\.system|os\.popen|subprocess\.\w+|system|exec|execSync|execFileSync|spawnSync|popen|Popen|check_output)\s*\(\s*[fF]?(?:"((?:\\.|[^"\\])*)"|'((?:\\.|[^'\\])*)'|` + "`([^`]*)`" + `)`)
	// subprocess.run(["rm", "-rf", "x"])
	callList = regexp.MustCompile(`(?:subprocess\.\w+|Popen|check_output|execFileSync|spawnSync|system|exec)\s*\(\s*\[\s*((?:["'][^"']*["']\s*,?\s*)+)\]`)
	quoted   = regexp.MustCompile(`["']([^"']*)["']`)
	backtick = regexp.MustCompile("`([^`]*)`|%x\\{([^}]*)\\}")
)

// embeddedShell pulls shell command strings out of inline program text, so
// `python3 -c "import os; os.system('rm -rf /')"` is checked as `rm -rf /`.
// It is a best-effort scan, not a Python parser.
func embeddedShell(code string) []string {
	if code == "" {
		return nil
	}
	var out []string
	for _, m := range callString.FindAllStringSubmatch(code, -1) {
		out = append(out, m[1]+m[2]+m[3])
	}
	for _, m := range callList.FindAllStringSubmatch(code, -1) {
		var words []string
		for _, q := range quoted.FindAllStringSubmatch(m[1], -1) {
			words = append(words, q[1])
		}
		out = append(out, strings.Join(words, " "))
	}
	for _, m := range backtick.FindAllStringSubmatch(code, -1) {
		out = append(out, m[1]+m[2])
	}
	return out
}
