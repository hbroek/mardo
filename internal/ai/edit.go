package ai

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxEditRunes limits the document the model rewrites: it has to write all of it back.
const MaxEditRunes = 60000

// Edit applies an instruction to a markdown document and returns the whole revised document.
// The reader reviews the result before anything is saved, so the model is told to change only
// what was asked, and a fence or a remark around the document is removed here.
func Edit(ctx context.Context, c Completer, title, current, instruction string) (string, error) {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return "", errors.New("describe the change first")
	}
	if utf8.RuneCountInString(instruction) > MaxQuestionRunes {
		return "", errors.New("that instruction is too long")
	}
	if utf8.RuneCountInString(current) > MaxEditRunes {
		return "", errors.New("this document is too long to change with AI; select less, or edit it by hand")
	}
	system := "You edit one markdown document for its author. Apply the author's instruction and reply with the complete revised document in Markdown, " +
		"and nothing else: no explanation, no code fence around it. Change only what the instruction asks for; keep every other line exactly as it is, " +
		"including the author's wording, headings, links and blank lines. If the instruction cannot be done to this document, reply with the document unchanged. " +
		injectionNote + "\n\nTitle: " + title + "\n\n--- document ---\n" + current
	reply, err := c.Complete(ctx, []Message{{Role: "system", Content: system}, {Role: "user", Content: instruction}})
	if err != nil {
		return "", err
	}
	return stripFence(reply), nil
}

// stripFence removes a code fence a model put around the whole document. A document that only
// contains fences of its own is left alone.
func stripFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") || !strings.HasSuffix(t, "```") {
		return keepEnd(s)
	}
	first := strings.IndexByte(t, '\n')
	if first < 0 {
		return keepEnd(s)
	}
	body := t[first+1 : len(t)-3]
	if strings.Contains(body, "```") && strings.Count(body, "```")%2 == 1 {
		return keepEnd(s) // an inner fence is unbalanced: these were the document's own
	}
	return keepEnd(strings.TrimRight(body, "\n"))
}

// keepEnd gives a document the one final newline files have.
func keepEnd(s string) string {
	return strings.TrimRight(s, "\n") + "\n"
}
