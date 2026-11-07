package domain

// Evidence says how we know an effect, strongest first.
type Evidence int

const (
	Estimated Evidence = iota // static reasoning only; never auto-runs
	Inspected                 // read from the real current state, read-only
	Previewed                 // the tool's own dry run
	Observed                  // measured by really running it in the sandbox
)

func (e Evidence) String() string {
	return [...]string{"ESTIMATED", "INSPECTED", "PREVIEWED", "OBSERVED"}[e]
}

// EffectKind names the part of the system an effect touches.
type EffectKind string

const (
	EffectFS      EffectKind = "files"
	EffectProcess EffectKind = "processes"
	EffectDisk    EffectKind = "disk"
	EffectNetwork EffectKind = "network"
	EffectReview  EffectKind = "review"
)

// Effect is one thing a command would do, with how sure we are.
type Effect struct {
	Kind     EffectKind `json:"kind"`
	Evidence Evidence   `json:"evidence"`
	Summary  string     `json:"summary"`
	Details  []string   `json:"details,omitempty"`
	// MinLevel is the lowest risk level this effect justifies. Effects can raise
	// a command's level, never lower it.
	MinLevel Level `json:"min_level"`
}

// MarshalJSON writes the evidence as its name.
func (e Evidence) MarshalJSON() ([]byte, error) { return []byte(`"` + e.String() + `"`), nil }

// Decision is what SafeShell does with a command.
type Decision int

const (
	Allow     Decision = iota // run it
	Confirm                   // show the impact and ask
	Block                     // refuse unless the user overrides
	HardBlock                 // refuse; overriding needs a typed phrase
)

func (d Decision) String() string {
	return [...]string{"allow", "confirm", "block", "hard-block"}[d]
}

// MarshalJSON writes the decision as its name.
func (d Decision) MarshalJSON() ([]byte, error) { return []byte(`"` + d.String() + `"`), nil }
