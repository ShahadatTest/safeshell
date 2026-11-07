// Package install adds SafeShell's hook to a shell's startup file, and removes it.
package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	begin = "# >>> safeshell >>>"
	end   = "# <<< safeshell <<<"
)

// Shell is a supported shell and where its startup file is.
type Shell struct {
	Name string // bash or zsh
	RC   string
}

// Detect picks the shell from $SHELL (a path such as /bin/zsh). home is the user's home folder.
func Detect(shellEnv, home string) (Shell, error) {
	switch filepath.Base(shellEnv) {
	case "bash":
		return Shell{"bash", filepath.Join(home, ".bashrc")}, nil
	case "zsh":
		return Shell{"zsh", filepath.Join(home, ".zshrc")}, nil
	}
	return Shell{}, errors.New("only bash and zsh are supported (your shell is " + filepath.Base(shellEnv) + ")")
}

func block(sh Shell, bin string) string {
	return begin + "\n[ -x " + quote(bin) + " ] && eval \"$(" + quote(bin) + " hook " + sh.Name + ")\"\n" + end + "\n"
}

func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// Add puts the hook block in the startup file. It is idempotent, keeps the rest
// of the file untouched, and writes a .safeshell-backup copy the first time.
// changed is false when the hook was already there.
func Add(sh Shell, bin string) (changed bool, err error) {
	old, err := os.ReadFile(sh.RC)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	text := string(old)
	if strings.Contains(text, begin) {
		return false, nil
	}
	if len(old) > 0 {
		if _, e := os.Stat(sh.RC + ".safeshell-backup"); e != nil {
			if err := os.WriteFile(sh.RC+".safeshell-backup", old, 0o600); err != nil {
				return false, err
			}
		}
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n"
	}
	return true, os.WriteFile(sh.RC, []byte(text+block(sh, bin)), 0o644)
}

// Remove deletes the block, leaving everything else as it was.
func Remove(sh Shell) (changed bool, err error) {
	old, err := os.ReadFile(sh.RC)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	text := string(old)
	i := strings.Index(text, begin)
	j := strings.Index(text, end)
	if i < 0 || j < i {
		return false, nil
	}
	j += len(end)
	if j < len(text) && text[j] == '\n' {
		j++
	}
	out := strings.TrimRight(text[:i], "\n")
	rest := text[j:]
	switch {
	case out == "":
		out = rest
	case rest != "":
		out += "\n" + rest
	default:
		out += "\n"
	}
	return true, os.WriteFile(sh.RC, []byte(out), 0o644)
}

// Installed reports whether the block is in the startup file.
func Installed(sh Shell) bool {
	b, err := os.ReadFile(sh.RC)
	return err == nil && strings.Contains(string(b), begin)
}
