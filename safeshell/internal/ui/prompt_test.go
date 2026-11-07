package ui

import (
	"bufio"
	"strings"
	"testing"

	"safeshell/internal/domain"
)

var alts = []domain.Alternative{{Command: "trash proj"}, {Command: "mv proj /tmp/x"}}

func choose(input string, d domain.Decision) (string, bool) {
	var out strings.Builder
	return Choose(bufio.NewReader(strings.NewReader(input)), &out, "rm -rf proj", d, alts)
}

func TestChoose(t *testing.T) {
	for _, tc := range []struct {
		name, in string
		d        domain.Decision
		want     string
		ok       bool
	}{
		{"allow runs", "", domain.Allow, "rm -rf proj", true},
		{"confirm yes", "y\n", domain.Confirm, "rm -rf proj", true},
		{"confirm default no", "\n", domain.Confirm, "", false},
		{"confirm eof", "", domain.Confirm, "", false},
		{"block default cancels", "\n", domain.Block, "", false},
		{"block cancel", "c\n", domain.Block, "", false},
		{"block alt 2", "2\n", domain.Block, "mv proj /tmp/x", true},
		{"block alt out of range then cancel", "9\nc\n", domain.Block, "", false},
		{"block do anyway needs yes", "d\nyes\n", domain.Block, "rm -rf proj", true},
		{"block do anyway declined", "d\nno\n", domain.Block, "", false},
		{"block garbage then eof", "zzz\n", domain.Block, "", false},
		{"hard needs phrase", "yes\n", domain.HardBlock, "", false},
		{"hard with phrase", HardPhrase + "\n", domain.HardBlock, "rm -rf proj", true},
	} {
		got, ok := choose(tc.in, tc.d)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: got (%q,%v), want (%q,%v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
