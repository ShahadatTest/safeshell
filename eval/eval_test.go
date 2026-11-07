package eval

import (
	"testing"

	"github.com/samarthsinh2660/safeshell/internal/domain"
)

func TestCorpusLoads(t *testing.T) {
	rows, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 200 {
		t.Errorf("corpus has %d rows, want at least 200", len(rows))
	}
	seen := map[domain.Level]int{}
	for _, r := range rows {
		seen[r.Want]++
	}
	for l := domain.R0; l <= domain.R4; l++ {
		if seen[l] < 10 {
			t.Errorf("only %d rows for %s, want at least 10", seen[l], l)
		}
	}
}

func TestClassifierMatchesCorpus(t *testing.T) {
	rows, err := LoadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	m := Evaluate(rows)
	t.Logf("accuracy %d/%d (%.1f%%), %d known gaps", m.Correct, m.Total, 100*float64(m.Correct)/float64(m.Total), len(m.Gaps))
	for _, g := range m.Gaps {
		t.Logf("known gap: %-45q want %s got %s  (%s)", g.Command, g.Want, g.Got, g.Note)
	}
	for _, r := range m.Mismatches {
		t.Errorf("%-50q want %s got %s", r.Command, r.Want, r.Got)
	}
}

// The project's headline safety claim: nothing rated R3 or R4 in the corpus is
// ever let through at a lower level.
func TestNoDangerousFalseNegatives(t *testing.T) {
	rows, _ := LoadCorpus()
	for _, r := range Evaluate(rows).SevereMisses {
		t.Errorf("dangerous command under-rated: %q want %s got %s", r.Command, r.Want, r.Got)
	}
}

// The held-out set was written by someone who had not seen the rules. Results at
// the time of writing: 92.3% exact, all but one miss within one level (a POST labelled a read), and the six
// dangerous-side misses are all debatable labels (sudo-raising of an install or
// kill, a recursive chown of your own tree, and shutdown/reboot rated R3 where
// the labeller said R4). These floors stop the rules from drifting back.
func TestHeldoutReport(t *testing.T) {
	rows, err := LoadHeldout()
	if err != nil {
		t.Fatal(err)
	}
	m := Evaluate(rows)
	t.Logf("held-out: %d of %d exact (%.1f%%), %d dangerous under-ratings", m.Correct, m.Total, 100*float64(m.Correct)/float64(m.Total), len(m.SevereMisses))
	far := 0
	for _, r := range m.Mismatches {
		t.Logf("  want %s got %s  %s   [%s]", r.Want, r.Got, r.Command, r.Note)
		if d := int(r.Want) - int(r.Got); d > 1 || d < -1 {
			far++
		}
	}
	if m.Total < 200 {
		t.Errorf("only %d held-out rows", m.Total)
	}
	if acc := float64(m.Correct) / float64(m.Total); acc < 0.90 {
		t.Errorf("held-out accuracy fell to %.1f%%", acc*100)
	}
	if far > 1 { // the one known case is a POST the labeller called a read
		t.Errorf("%d held-out rows are off by more than one level", far)
	}
	if len(m.SevereMisses) > 6 {
		t.Errorf("%d dangerous under-ratings, up from 6", len(m.SevereMisses))
	}
}
