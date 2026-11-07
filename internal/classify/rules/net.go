package rules

import (
	"strings"

	"github.com/samarthsinh2660/safeshell/internal/shellparse"
)

func ruleCurl(c shellparse.Command, f flags) []Finding {
	if c.Name == "curl" && (f.hasAny("-X", "--request") && (f.hasAny("POST", "PUT", "DELETE", "PATCH") || strings.Contains(f.joined(), "POST") || strings.Contains(f.joined(), "DELETE")) ||
		f.hasAny("-d", "--data", "-F", "--form", "-T", "--upload-file", "--data-raw", "--data-binary")) {
		return one(fnd(R2, c, "Sends data to, or changes something on, a remote server. That cannot be undone from here."))
	}
	toStdout := c.Name == "curl" && !f.hasAny("-o", "--output", "-O", "--remote-name", "-J") ||
		c.Name == "wget" && (f.hasAny("-O-", "-qO-") || strings.Contains(f.joined(), "-O -"))
	if toStdout {
		return one(fnd(R0, c, "Fetches a page and prints it. Nothing is saved."))
	}
	return one(fnd(R1, c, "Downloads a file from the network."))
}

func ruleRsync(c shellparse.Command, f flags) []Finding {
	if f.has('n', "dry-run") {
		return one(fnd(R0, c, "Dry run: lists what would be copied without copying anything."))
	}
	for _, a := range f.all {
		if strings.HasPrefix(a, "--delete") || a == "--remove-source-files" {
			return one(fnd(R3, c, "Deletes files at the destination (or source) that do not match. A wrong path wipes a folder.").
				WithAlt(c.String()+" --dry-run", "lists what would change, without changing it"))
		}
	}
	return one(fnd(R1, c, "Copies and syncs files."))
}

func ruleSsh(c shellparse.Command, f flags) []Finding {
	if len(f.pos) > 1 {
		return one(fnd(R0, c, "Runs a command on another machine. That command is checked on its own."))
	}
	return one(fnd(R1, c, "Opens a session on another machine. What you type there is not checked."))
}
