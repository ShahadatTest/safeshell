// Package shellhook holds the code a shell sources to route every command line
// through `ss guard` before it runs.
package shellhook

import "fmt"

const bash = `# SafeShell hook for bash. Add to ~/.bashrc:  eval "$(ss hook bash)"
__ss_guard() {
  [[ -n "$SAFESHELL_OFF" || -z "${READLINE_LINE//[[:space:]]/}" ]] && return
  local out rc tty
  tty=$(stty -g </dev/tty) && stty sane </dev/tty # readline leaves the terminal raw, so Enter would not end an answer
  out=$(command ss guard --cwd "$PWD" -- "$READLINE_LINE" </dev/tty 2>/dev/tty)
  rc=$?
  [[ -n $tty ]] && stty "$tty" </dev/tty
  if (( rc == 0 )); then READLINE_LINE=$out; else READLINE_LINE=; fi
  READLINE_POINT=${#READLINE_LINE}
}
if [[ $- == *i* ]]; then
  bind -x '"\C-x\C-s": __ss_guard'
  bind '"\C-m": "\C-x\C-s\C-j"'
fi
`

const zsh = `# SafeShell hook for zsh. Add to ~/.zshrc:  eval "$(ss hook zsh)"
__ss_accept_line() {
  if [[ -z "$SAFESHELL_OFF" && -n "${BUFFER//[[:space:]]/}" ]]; then
    local out rc tty
    tty=$(stty -g </dev/tty) && stty sane </dev/tty
    out=$(command ss guard --cwd "$PWD" -- "$BUFFER" </dev/tty 2>/dev/tty)
    rc=$?
    [[ -n $tty ]] && stty "$tty" </dev/tty
    if (( rc == 0 )); then BUFFER=$out; else BUFFER=; fi
  fi
  zle .accept-line
}
zle -N accept-line __ss_accept_line
`

// Script returns the hook for the named shell.
func Script(shell string) (string, error) {
	switch shell {
	case "bash":
		return bash, nil
	case "zsh":
		return zsh, nil
	}
	return "", fmt.Errorf("unsupported shell %q (use bash or zsh)", shell)
}
