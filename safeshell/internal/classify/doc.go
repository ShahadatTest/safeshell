// Package classify rates a parsed command by reversibility (R0-R4).
//
// Pure and fast: no I/O, no processes, no network. It must stay cheap enough to
// run inside the shell hook in a few milliseconds. May import: domain, shellparse.
package classify
