package risk

import "strings"

// flags is a command's arguments split into options and operands.
type flags struct {
	short map[byte]bool
	long  map[string]string // --name[=value]
	pos   []string          // operands, in order
	all   []string
}

func parseFlags(args []string) flags {
	f := flags{short: map[byte]bool{}, long: map[string]string{}, all: args}
	endOpts := false
	for _, a := range args {
		switch {
		case endOpts || a == "-" || !strings.HasPrefix(a, "-"):
			f.pos = append(f.pos, a)
		case a == "--":
			endOpts = true
		case strings.HasPrefix(a, "--"):
			k, v, _ := strings.Cut(a[2:], "=")
			f.long[k] = v
		default:
			for i := 1; i < len(a); i++ {
				f.short[a[i]] = true
			}
		}
	}
	return f
}

func (f flags) has(short byte, long ...string) bool {
	if short != 0 && f.short[short] {
		return true
	}
	for _, l := range long {
		if _, ok := f.long[l]; ok {
			return true
		}
	}
	return false
}

func (f flags) hasAny(words ...string) bool {
	for _, a := range f.all {
		for _, w := range words {
			if a == w {
				return true
			}
		}
	}
	return false
}

// sub returns the first operand (the sub-command of git, docker, apt, ...).
func (f flags) sub() string {
	if len(f.pos) == 0 {
		return ""
	}
	return f.pos[0]
}

func (f flags) joined() string { return strings.Join(f.all, " ") }
