package domain

import (
	"encoding/json"
	"testing"
)

func TestLevelEvidenceDecisionSurviveAJSONRoundTrip(t *testing.T) {
	in := struct {
		L Level
		E Evidence
		D Decision
	}{R3, Observed, HardBlock}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		L Level
		E Evidence
		D Decision
	}
	if err := json.Unmarshal(b, &out); err != nil || out != in {
		t.Fatalf("%s -> %+v %v", b, out, err)
	}
	for _, bad := range []string{`{"L":"R9"}`, `{"E":"GUESSED"}`, `{"D":"maybe"}`, `{"L":3}`} {
		if json.Unmarshal([]byte(bad), &out) == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}
