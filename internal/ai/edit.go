package ai

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// The reader's cursor, or selected text, is shown to the model with these marks; they are removed from what comes back.
const (
	CursorMark     = "⟦CURSOR⟧"
	SelectionOpen  = "⟦SELECTION⟧"
	SelectionClose = "⟦/SELECTION⟧"
)

// cannotPrefix starts a reply that says why the instruction cannot be done, instead of a document.
const cannotPrefix = "CANNOT:"

// MaxEditRunes limits the document the model rewrites: it has to write all of it back.
const MaxEditRunes = 60000

// Edit applies an instruction to a markdown document and returns the whole revised document.
// The reader reviews the result before anything is saved, so the model is told to change only
// what was asked, and a fence or a remark around the document is removed here. When the model
// says the instruction cannot be done, the document comes back unchanged with its reason in note.
func Edit(ctx context.Context, c Completer, title, current string, history []Message, instruction string) (text, note string, err error) {
	return edit(ctx, c, time.Now(), title, current, history, instruction)
}

func edit(ctx context.Context, c Completer, now time.Time, title, current string, history []Message, instruction string) (string, string, error) {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return "", "", errors.New("describe the change first")
	}
	if utf8.RuneCountInString(instruction) > MaxQuestionRunes {
		return "", "", errors.New("that instruction is too long")
	}
	if utf8.RuneCountInString(current) > MaxEditRunes {
		return "", "", errors.New("this page is too long to change with AI; edit it by hand")
	}
	system := "You edit one markdown document for its author. Apply the author's instruction and reply with the complete revised document in Markdown, " +
		"and nothing else: no explanation, no code fence around it. Change only what the instruction asks for; keep every other line exactly as it is, " +
		"including the author's wording, headings, links and blank lines. " +
		"If the instruction cannot be done to this document, reply with one line that starts with " + cannotPrefix + " and says why, and nothing else. " +
		"The current date and time is " + now.Format("Monday 2006-01-02 15:04:05 -07:00") + " (ISO 8601: " + now.Format(time.RFC3339) + "); use it when asked for today's date or the time. " +
		"The author's cursor may be shown in the document as " + CursorMark + ", and selected text as " + SelectionOpen + "..." + SelectionClose + ": " +
		"\"here\" and \"at the cursor\" mean that spot, and \"this\" or \"the selection\" mean the selected text. Never write these marks in your reply. " +
		"Earlier messages of this conversation may be listed below, so that \"it\", \"again\" and \"also\" make sense; the document already contains the changes that were accepted. " +
		injectionNote + transcript(history) + "\n\nTitle: " + title + "\n\n--- document ---\n" + current
	reply, err := c.Complete(ctx, []Message{{Role: "system", Content: system}, {Role: "user", Content: instruction}})
	if err != nil {
		return "", "", err
	}
	if t := strings.TrimSpace(reply); strings.HasPrefix(t, cannotPrefix) {
		return unmark(current), strings.TrimSpace(strings.TrimPrefix(t, cannotPrefix)), nil
	}
	return unmark(stripFence(reply)), "", nil
}

// unmark removes the cursor and selection marks.
func unmark(s string) string {
	return strings.NewReplacer(CursorMark, "", SelectionOpen, "", SelectionClose, "").Replace(s)
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

// transcript lists the earlier messages of the conversation, for the system message. The answers are
// cut short: they are remarks about a change, not documents.
func transcript(history []Message) string {
	if len(history) > MaxTurns {
		history = history[len(history)-MaxTurns:]
	}
	var b strings.Builder
	for _, m := range history {
		text := strings.TrimSpace(m.Content)
		if text == "" || (m.Role != "user" && m.Role != "assistant") {
			continue
		}
		who := "Author"
		if m.Role == "assistant" {
			who, text = "You", Truncate(text, 300)
		}
		if b.Len() == 0 {
			b.WriteString("\n\n--- earlier messages ---")
		}
		b.WriteString("\n" + who + ": " + strings.ReplaceAll(text, "\n", " "))
	}
	return b.String()
}
