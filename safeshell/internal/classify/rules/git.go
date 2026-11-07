package rules

import (
	"strings"

	"safeshell/internal/shellparse"
)

func ruleGit(c shellparse.Command, f flags) []Finding {
	switch f.sub() {
	case "push":
		switch {
		case f.hasAny("--force-with-lease") || f.long["force-with-lease"] != "" || f.has(0, "force-with-lease"):
			return one(fnd(R2, c, "Rewrites history on the remote, but refuses if someone else pushed in the meantime."))
		case f.has('f', "force") || f.hasAny("--delete", "-d") || anyPrefix(f.pos, ":"):
			return one(fnd(R3, c, "Overwrites or deletes history on the shared remote. Other people's work can be lost.").
				WithAlt(strings.Replace(c.String(), "--force", "--force-with-lease", 1), "refuses if someone else pushed in the meantime"))
		}
		return one(fnd(R2, c, "Publishes commits to a shared remote. Hard to take back once others pull them."))
	case "reset":
		if f.hasAny("--hard") {
			return one(fnd(R2, c, "Throws away all uncommitted changes. Committed work stays recoverable through the reflog.").
				WithAlt("git stash", "saves your changes first so they can be brought back"))
		}
		return one(fnd(R1, c, "Moves the branch pointer. Files are kept."))
	case "clean":
		if f.has('n', "dry-run") {
			return one(fnd(R0, c, "Only lists the untracked files it would remove."))
		}
		if f.has('f', "force") {
			return one(fnd(R3, c, "Deletes untracked files that git does not track, so they cannot be restored.").
				WithAlt("git clean -n"+strings.TrimPrefix(c.String(), "git clean"), "a dry run that lists what would be deleted"))
		}
		return one(fnd(R1, c, "Refuses to delete without -f."))
	case "checkout", "restore", "switch":
		if f.hasAny(".") || f.hasAny("--", "-f", "--force", "--discard-changes") && f.hasAny(".") {
			return one(fnd(R2, c, "Discards uncommitted changes in the working folder."))
		}
		return one(fnd(R1, c, "Switches branches or restores files."))
	case "branch":
		if f.hasAny("-D") || (f.has('d', "delete") && f.has('f', "force")) {
			return one(fnd(R2, c, "Deletes a branch even if it is not merged. Recoverable through the reflog."))
		}
		return one(fnd(R0, c, "Lists or creates branches."))
	case "rebase", "filter-branch", "filter-repo", "commit-tree":
		return one(fnd(R2, c, "Rewrites commit history."))
	case "gc", "reflog", "prune":
		if f.hasAny("--prune=now", "--expire=now", "--expire-unreachable=now") {
			return one(fnd(R3, c, "Permanently erases the safety net (reflog and unreachable commits) used to recover lost work."))
		}
		return one(fnd(R1, c, "Housekeeping."))
	case "stash":
		if len(f.pos) > 1 && (f.pos[1] == "drop" || f.pos[1] == "clear") {
			return one(fnd(R2, c, "Deletes saved stashes."))
		}
		return one(fnd(R1, c, "Saves or restores changes."))
	case "status", "log", "diff", "show", "fetch", "blame", "ls-files", "describe", "remote", "config", "rev-parse", "grep", "shortlog", "ls-remote", "":
		return one(fnd(R0, c, "Only reads repository information."))
	}
	return one(fnd(R1, c, "Changes the local repository. Reversible through git history."))
}

func anyPrefix(ss []string, p string) bool {
	for _, s := range ss {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
