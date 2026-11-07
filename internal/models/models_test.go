package models

import (
	"regexp"
	"strings"
	"testing"
)

func TestCatalogEntriesAreCompleteAndUnique(t *testing.T) {
	sha := regexp.MustCompile(`^[0-9a-f]{64}$`)
	seen := map[string]bool{}
	for _, m := range Catalog {
		if seen[m.ID] {
			t.Errorf("duplicate id %s", m.ID)
		}
		seen[m.ID] = true
		if !sha.MatchString(m.SHA) || m.Size < 1<<28 || !strings.HasPrefix(m.URL, "https://huggingface.co/") || !strings.HasSuffix(m.File, ".gguf") || !strings.HasSuffix(m.URL, m.File) {
			t.Errorf("%s is incomplete: %+v", m.ID, m)
		}
	}
	if _, ok := Find(Default); !ok {
		t.Error("the default model is not in the catalog")
	}
	if _, ok := Find("nope"); ok {
		t.Error("Find invented a model")
	}
}
