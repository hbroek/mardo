package ai

import (
	"strings"
	"testing"
)

func TestEditSendsDocumentAndInstruction(t *testing.T) {
	var got []Message
	c := completeFunc(func(m []Message) string { got = m; return "# New\n" })
	out, err := Edit(t.Context(), c, "Note", "# Old\n", "  rename it ")
	if err != nil || out != "# New\n" {
		t.Fatalf("Edit = %q, %v", out, err)
	}
	if len(got) != 2 || got[1].Content != "rename it" || !strings.Contains(got[0].Content, "# Old") || !strings.Contains(got[0].Content, "untrusted") {
		t.Fatalf("messages = %+v", got)
	}
}

func TestEditLimits(t *testing.T) {
	c := completeFunc(func([]Message) string { return "x" })
	if _, err := Edit(t.Context(), c, "", "doc", " "); err == nil {
		t.Error("empty instruction accepted")
	}
	if _, err := Edit(t.Context(), c, "", strings.Repeat("x", MaxEditRunes+1), "do"); err == nil {
		t.Error("huge document accepted")
	}
}

func TestStripFence(t *testing.T) {
	for in, want := range map[string]string{
		"```markdown\n# A\n\ntext\n```":      "# A\n\ntext\n",
		"# A\n":                              "# A\n",
		"# A":                                "# A\n",
		"```go\nfmt.Println()\n```\n\nafter": "```go\nfmt.Println()\n```\n\nafter\n",
		"```\ncode\n```":                     "code\n", // a bare fenced reply is treated as the wrapper
	} {
		if got := stripFence(in); got != want {
			t.Errorf("stripFence(%q) = %q, want %q", in, got, want)
		}
	}
}
