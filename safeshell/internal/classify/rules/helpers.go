package rules

import (
	"safeshell/internal/domain"
	"safeshell/internal/shellparse"
)

// Local aliases keep the rule tables readable.
type Finding = domain.Finding

const (
	R0 = domain.R0
	R1 = domain.R1
	R2 = domain.R2
	R3 = domain.R3
	R4 = domain.R4
)

// A rule rates one command. It returns one finding per problem it sees.
type rule func(c shellparse.Command, f flags) []Finding

func fnd(l domain.Level, c shellparse.Command, reason string) Finding {
	return Finding{Level: l, Command: c.String(), Reason: reason}
}

func one(f Finding) []Finding { return []Finding{f} }

// fixed returns a rule that always gives the same answer.
func fixed(l domain.Level, reason string) rule {
	return func(c shellparse.Command, _ flags) []Finding { return one(fnd(l, c, reason)) }
}
