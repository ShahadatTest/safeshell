package report

import (
	"fmt"
	"strings"

	"safeshell/internal/domain"
	"safeshell/internal/sandbox"
)

// FileEffect turns an observed file diff into an Effect: what happened, the
// biggest changes, and the level its impact justifies.
func FileEffect(d sandbox.FileDiff) domain.Effect {
	t := d.Totals()
	im := Score(d)
	e := domain.Effect{Kind: domain.EffectFS, Evidence: domain.Observed, MinLevel: im.Level}
	var parts []string
	if t.Deleted > 0 {
		parts = append(parts, fmt.Sprintf("delete %s (%s)", Files(t.Deleted), HumanBytes(t.DeletedBytes)))
	}
	if t.Modified > 0 {
		parts = append(parts, "change "+Files(t.Modified))
	}
	if t.Created > 0 {
		parts = append(parts, "create "+Files(t.Created))
	}
	if len(parts) == 0 {
		e.Summary = "changes no files"
	} else {
		e.Summary = "would " + strings.Join(parts, ", ")
	}
	if d.Truncated {
		e.Summary += " (counting stopped at a limit; the real total is larger)"
	}
	for _, r := range im.Reasons {
		e.Details = append(e.Details, "because it "+r)
	}
	const max = 8
	for i, c := range d.Changes {
		if i == max {
			e.Details = append(e.Details, fmt.Sprintf("... and %d more changes", len(d.Changes)-max))
			break
		}
		slash := ""
		if c.IsDir {
			slash = "/"
		}
		line := fmt.Sprintf("%s /%s%s", c.Kind, c.Path, slash)
		if c.IsDir && c.Files > 0 {
			line += " (" + Files(c.Files) + ")"
		}
		e.Details = append(e.Details, line)
	}
	return e
}
