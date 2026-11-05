package ai

import (
	"context"
	"strings"
	"unicode/utf8"
)

// Native is a Chatter and Completer for a model that runs inside the app itself, such as the one on an iPhone.
// Such a model reads only a few thousand words at once, so what is sent to it is cut to MaxRunes first.
type Native struct {
	Run      func(ctx context.Context, messages []Message) (string, error)
	MaxRunes int // how much text the model takes at once; 0 sends it all
}

// Chat sends a system and a user message and returns the reply text.
func (n Native) Chat(ctx context.Context, system, user string) (string, error) {
	return n.Complete(ctx, []Message{{Role: "system", Content: system}, {Role: "user", Content: user}})
}

// Complete sends the conversation, cut to fit, and returns the reply text.
func (n Native) Complete(ctx context.Context, messages []Message) (string, error) {
	reply, err := n.Run(ctx, Fit(messages, n.MaxRunes))
	return strings.TrimSpace(reply), err
}

const cutNote = "\n[…the rest was cut to fit the model]"

// Fit shortens a conversation to at most max runes: the oldest turns go first (the first message, which holds the
// instructions and often the page, and the last, the question, stay), then the longest message is cut at its end.
func Fit(messages []Message, max int) []Message {
	out := append([]Message(nil), messages...)
	if max <= 0 {
		return out
	}
	total := func() int {
		n := 0
		for _, m := range out {
			n += utf8.RuneCountInString(m.Content)
		}
		return n
	}
	for total() > max && len(out) > 2 {
		out = append(out[:1], out[2:]...)
	}
	if excess := total() - max; excess > 0 {
		longest := 0
		for i, m := range out {
			if utf8.RuneCountInString(m.Content) > utf8.RuneCountInString(out[longest].Content) {
				longest = i
			}
		}
		runes := []rune(out[longest].Content)
		keep := len(runes) - excess - utf8.RuneCountInString(cutNote)
		if keep < 0 {
			keep = 0
		}
		out[longest].Content = string(runes[:keep]) + cutNote
	}
	return out
}
