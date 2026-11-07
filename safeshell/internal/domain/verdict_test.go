package domain

import (
	"encoding/json"
	"testing"
)

func TestLevelNames(t *testing.T) {
	want := []struct{ s, label string }{{"R0", "Read-only"}, {"R1", "Reversible"}, {"R2", "Recoverable"}, {"R3", "Irreversible"}, {"R4", "Catastrophic"}}
	for i, w := range want {
		l := Level(i)
		if l.String() != w.s || l.Label() != w.label {
			t.Errorf("level %d = %s/%s, want %s/%s", i, l, l.Label(), w.s, w.label)
		}
	}
}

func TestLevelJSON(t *testing.T) {
	b, _ := json.Marshal(R3)
	if string(b) != `"R3"` {
		t.Errorf("got %s", b)
	}
}

func TestMaxLevel(t *testing.T) {
	if MaxLevel(nil) != R0 {
		t.Error("no findings should be R0")
	}
	fs := []Finding{{Level: R1}, {Level: R4}, {Level: R2}}
	if MaxLevel(fs) != R4 {
		t.Error("want R4")
	}
}

func TestWithAltDoesNotMutateOriginal(t *testing.T) {
	f := Finding{Level: R3}
	g := f.WithAlt("trash-put x", "recoverable")
	if f.Alt != nil || g.Alt == nil || g.Alt.Command != "trash-put x" {
		t.Errorf("WithAlt must return a copy: %+v %+v", f, g)
	}
}

func TestEvidenceAndDecisionNames(t *testing.T) {
	if Observed.String() != "OBSERVED" || Estimated.String() != "ESTIMATED" || Inspected.String() != "INSPECTED" || Previewed.String() != "PREVIEWED" {
		t.Error("evidence names")
	}
	if Allow.String() != "allow" || HardBlock.String() != "hard-block" {
		t.Error("decision names")
	}
	b, _ := json.Marshal(Effect{Kind: EffectFS, Evidence: Observed})
	if string(b) != `{"kind":"files","evidence":"OBSERVED","summary":"","min_level":"R0"}` {
		t.Errorf("effect JSON = %s", b)
	}
	d, _ := json.Marshal(Confirm)
	if string(d) != `"confirm"` {
		t.Errorf("decision JSON = %s", d)
	}
	if !(Observed > Previewed && Previewed > Inspected && Inspected > Estimated) {
		t.Error("evidence must be ordered strongest last")
	}
}
