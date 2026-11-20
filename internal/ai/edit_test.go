package ai

import (
	"strings"
	"testing"
	"time"
)

func TestEditSendsDocumentAndInstruction(t *testing.T) {
	var got []Message
	c := completeFunc(func(m []Message) string { got = m; return "# New\n" })
	out, note, err := Edit(t.Context(), c, "Note", "# Old\n", nil, "  rename it ")
	if err != nil || out != "# New\n" || note != "" {
		t.Fatalf("Edit = %q, %q, %v", out, note, err)
	}
	if len(got) != 2 || got[1].Content != "rename it" || !strings.Contains(got[0].Content, "# Old") || !strings.Contains(got[0].Content, "untrusted") {
		t.Fatalf("messages = %+v", got)
	}
}

func TestEditLimits(t *testing.T) {
	c := completeFunc(func([]Message) string { return "x" })
	if _, _, err := Edit(t.Context(), c, "", "doc", nil, " "); err == nil {
		t.Error("empty instruction accepted")
	}
	if _, _, err := Edit(t.Context(), c, "", strings.Repeat("x", MaxEditRunes+1), nil, "do"); err == nil {
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

func TestEditKnowsTheDateAndTheCursor(t *testing.T) {
	var got []Message
	c := completeFunc(func(m []Message) string { got = m; return "Hi " + CursorMark + "2026-11-20\n" })
	now := time.Date(2026, 11, 20, 10, 15, 0, 0, time.FixedZone("EST", -5*3600))
	out, _, err := edit(t.Context(), c, now, "", "Hi "+CursorMark+"\n", nil, "append today's date")
	if err != nil || out != "Hi 2026-11-20\n" {
		t.Fatalf("out = %q, %v", out, err)
	}
	if !strings.Contains(got[0].Content, "2026-11-20T10:15:00-05:00") || !strings.Contains(got[0].Content, CursorMark) {
		t.Fatalf("system message lacks the date or the cursor: %s", got[0].Content)
	}
}

func TestEditCannotGivesTheReason(t *testing.T) {
	c := completeFunc(func([]Message) string { return "CANNOT: there is no table to sort.\n" })
	out, note, err := Edit(t.Context(), c, "", "# Doc\n", nil, "sort the table")
	if err != nil || out != "# Doc\n" || note != "there is no table to sort." {
		t.Fatalf("out = %q, note = %q, err = %v", out, note, err)
	}
}

func TestEditSendsEarlierMessages(t *testing.T) {
	var got []Message
	c := completeFunc(func(m []Message) string { got = m; return "x" })
	h := []Message{{Role: "user", Content: "add a heading"}, {Role: "assistant", Content: "I drafted the change."}}
	if _, _, err := Edit(t.Context(), c, "", "doc", h, "make it bold"); err != nil {
		t.Fatal(err)
	}
	sys := got[0].Content
	if !strings.Contains(sys, "Author: add a heading") || !strings.Contains(sys, "You: I drafted the change.") {
		t.Fatalf("earlier messages missing: %s", sys)
	}
}
