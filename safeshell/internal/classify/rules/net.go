package rules

import (
	"strings"

	"safeshell/internal/shellparse"
)

func ruleCurl(c shellparse.Command, f flags) []Finding {
	if c.Name == "curl" && (f.hasAny("-X", "--request") && (f.hasAny("POST", "PUT", "DELETE", "PATCH") || strings.Contains(f.joined(), "POST") || strings.Contains(f.joined(), "DELETE")) ||
		f.hasAny("-d", "--data", "-F", "--form", "-T", "--upload-file", "--data-raw", "--data-binary")) {
		return one(fnd(R2, c, "Sends data to, or changes something on, a remote server. That cannot be undone from here."))
	}
	return one(fnd(R1, c, "Downloads from the network."))
}

func ruleRsync(c shellparse.Command, f flags) []Finding {
	for _, a := range f.all {
		if strings.HasPrefix(a, "--delete") || a == "--remove-source-files" {
			return one(fnd(R3, c, "Deletes files at the destination (or source) that do not match. A wrong path wipes a folder.").
				WithAlt(c.String()+" --dry-run", "lists what would change, without changing it"))
		}
	}
	return one(fnd(R1, c, "Copies and syncs files."))
}
