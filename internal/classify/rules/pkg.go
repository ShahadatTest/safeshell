package rules

import (
	"github.com/samarthsinh2660/safeshell/internal/shellparse"
)

func rulePkg(c shellparse.Command, f flags) []Finding {
	if f.hasAny("-n", "--dry-run", "--simulate", "-s", "--just-print") {
		return one(fnd(R0, c, "Dry run: shows what would change without changing it."))
	}
	switch f.sub() {
	case "remove", "purge", "autoremove", "erase", "uninstall", "-R", "-Rs", "-Rns", "-r", "-P":
		return one(fnd(R2, c, "Removes software, which can also remove things that depend on it.").WithAlt(c.String()+" --dry-run", "shows what would be removed"))
	case "list", "search", "show", "info", "policy", "-l", "-L", "-S", "-s", "check", "freeze":
		return one(fnd(R0, c, "Only reads package information."))
	}
	return one(fnd(R1, c, "Installs or updates software."))
}

func ruleNpm(c shellparse.Command, f flags) []Finding {
	switch f.sub() {
	case "uninstall", "remove", "rm", "unpublish", "deprecate":
		return one(fnd(R2, c, "Removes or unpublishes packages."))
	case "ls", "list", "ll", "la", "view", "info", "show", "outdated", "search", "ping", "whoami", "help", "explain", "why", "root", "prefix", "bin":
		return one(fnd(R0, c, "Only reads package information."))
	}
	return one(fnd(R1, c, "Runs package or project scripts, whose effects depend on the project."))
}
