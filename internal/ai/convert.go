package ai

import (
	"context"
	"strings"
)

// ConvertHTML asks the model to write a web page, given as simplified HTML, as Markdown. It is a rewrite of the
// page's own content, not a summary: the model is told to keep every heading, paragraph, list, table, link and
// image, and to leave out the page's navigation and boilerplate.
func ConvertHTML(ctx context.Context, c Chatter, title, pageURL, html string) (string, error) {
	system := "You convert a web page into clean Markdown for a markdown browser. The page is given as simplified HTML. " +
		"Keep all of the page's own content, in order: every heading, paragraph, list, table, link, image and code block, with the wording unchanged. " +
		"Use Markdown headings, lists, GitHub-style pipe tables, [text](url) links, ![alt](url) images and fenced code blocks. " +
		"Leave out what is not the page's content: menus, cookie notices, ads, share buttons, repeated footers. " +
		"Do not summarize, shorten, translate, reorder, explain or add anything, and do not invent links. " +
		"Reply with the Markdown only, without a code fence around it. " + injectionNote
	reply, err := c.Chat(ctx, system, pageMessage(title, pageURL, html))
	if err != nil {
		return "", err
	}
	return unfence(reply), nil
}

// unfence removes a code fence a model put around the whole reply.
func unfence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") || !strings.HasSuffix(t, "```") {
		return s
	}
	t = strings.TrimPrefix(t, "```")
	if i := strings.IndexByte(t, '\n'); i >= 0 && !strings.ContainsAny(t[:i], " \t") { // the language after the opening fence
		t = t[i+1:]
	}
	return strings.TrimSpace(strings.TrimSuffix(t, "```"))
}
