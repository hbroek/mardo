package core

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"mardo/internal/ai"
	"mardo/internal/fetch"
	"mardo/internal/render"
)

// Converted is an HTML page written as markdown.
type Converted struct {
	Title     string `json:"title"`
	Source    string `json:"source"`    // the markdown
	HTML      string `json:"html"`      // that markdown rendered, sanitized
	How       string `json:"how"`       // "rules" or "ai"
	Model     string `json:"model"`     // who wrote it, for "ai"
	Millis    int64  `json:"millis"`    // how long the model took, for "ai"
	Truncated bool   `json:"truncated"` // the page was longer than a model is sent, so the end is missing
}

// ConvertPage writes the HTML page at rawURL as markdown: how "rules" maps its tags to markdown syntax, how "ai"
// has the configured model rewrite it. Both keep the page's main content and leave out its navigation.
func (a *Core) ConvertPage(requestID, rawURL, how string) (Converted, error) {
	if how != "rules" && how != "ai" {
		return Converted{}, fmt.Errorf("unknown way to convert a page: %q", how)
	}
	var c aiProvider
	if how == "ai" {
		var err error
		if c, err = a.aiClient(); err != nil {
			return Converted{}, err
		}
	}
	ctx, done := a.aiContext(requestID)
	defer done()
	if rawURL == "" || strings.HasPrefix(rawURL, "bookmarks:") {
		return Converted{}, errors.New("there is no page to convert")
	}
	res, err := a.fetcher.Get(ctx, rawURL)
	if err != nil {
		return Converted{}, cancelled(ctx, err)
	}
	if res.Status >= 400 {
		return Converted{}, fmt.Errorf("the page answered %d", res.Status)
	}
	if fetch.IsMarkdown(res.ContentType) {
		return Converted{}, errors.New("this page is markdown already")
	}
	base, _ := url.Parse(res.URL)
	out := Converted{How: how}
	switch how {
	case "rules":
		out.Title, out.Source = render.ToMarkdown(res.Body, res.ContentType, base, false)
	case "ai":
		title, simple := render.SimplifiedHTML(res.Body, res.ContentType, base, false)
		if strings.TrimSpace(simple) == "" {
			return Converted{}, errors.New("the page has nothing to convert")
		}
		limit := ai.MaxPageRunes
		if n, ok := c.(ai.Native); ok {
			limit = n.MaxRunes
		}
		out.Truncated = len([]rune(simple)) > limit
		started := time.Now()
		reply, err := ai.ConvertHTML(ctx, c, title, res.URL, simple)
		if err != nil {
			return Converted{}, cancelled(ctx, err)
		}
		out.Title, out.Source = title, strings.TrimSpace(reply)+"\n"
		out.Model, out.Millis = a.aiSource(), time.Since(started).Milliseconds()
	}
	if strings.TrimSpace(out.Source) == "" {
		return Converted{}, errors.New("the page has no text to convert")
	}
	html, heading, err := render.Markdown([]byte(out.Source), base)
	if err != nil {
		return Converted{}, err
	}
	out.HTML = html
	if out.Title == "" {
		out.Title = heading
	}
	return out, nil
}
