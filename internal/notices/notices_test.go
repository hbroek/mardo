package notices

import (
	"os"
	"strings"
	"testing"
)

func TestSameAsRepositoryRoot(t *testing.T) {
	root, err := os.ReadFile("../../THIRD_PARTY_NOTICES.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(root) != Text {
		t.Fatal("internal/notices/THIRD_PARTY_NOTICES.md differs from the one in the repository root; copy it over")
	}
}

func TestListsTheMainLibraries(t *testing.T) {
	for _, name := range []string{"github.com/yuin/goldmark", "github.com/microcosm-cc/bluemonday", "modernc.org/sqlite"} {
		if !strings.Contains(Text, name) {
			t.Errorf("notices do not mention %s", name)
		}
	}
}

func TestReadingHasNoCodeBlocksButKeepsTheText(t *testing.T) {
	r := Reading()
	if strings.Contains(r, "~~~~") {
		t.Fatal("reading text still has the license fences")
	}
	if !strings.Contains(r, "Permission is hereby granted") || !strings.Contains(r, "modernc.org/sqlite") {
		t.Fatal("reading text lost the license text")
	}
}
