// Package policy decides what happens at each risk level. It is pure.
package policy

import "safeshell/internal/domain"

// Decide maps a final level to a decision. A command with any effect we could
// only estimate is never allowed to run unattended.
func Decide(level domain.Level, effects []domain.Effect) domain.Decision {
	d := domain.Allow
	switch level {
	case domain.R2:
		d = domain.Confirm
	case domain.R3:
		d = domain.Block
	case domain.R4:
		d = domain.HardBlock
	}
	if d == domain.Allow {
		for _, e := range effects {
			if e.Evidence == domain.Estimated {
				return domain.Confirm
			}
		}
	}
	return d
}

// FinalLevel is the highest of the rule level and what any effect justifies.
func FinalLevel(rule domain.Level, effects []domain.Effect) domain.Level {
	l := rule
	for _, e := range effects {
		l = max(l, e.MinLevel)
	}
	return l
}
