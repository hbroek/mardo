package browser

import (
	"context"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"markdown-browser/internal/fetch"
	"markdown-browser/internal/page"
)

// markdownURLs are the places the markdown version of a web page would be, best first, by the convention that a
// page at /about has one at /about.md, /docs/ at /docs/index.md and /guide.html at /guide.md. The root of a site
// may also have an /llms.txt, the site's overview for language models, which is markdown.
// Addresses with a query are dynamic and are left alone, as are those already ending in .md.
func markdownURLs(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" {
		return nil
	}
	with := func(p string) string {
		c := *u
		c.Path, c.RawPath, c.Fragment = p, "", ""
		return c.String()
	}
	p := u.Path
	switch ext := strings.ToLower(path.Ext(p)); {
	case p == "" || p == "/":
		return []string{with("/index.md"), with("/llms.txt")}
	case strings.HasSuffix(p, "/"):
		return []string{with(p + "index.md")}
	case ext == ".md" || ext == ".markdown":
		return nil
	case ext == ".html" || ext == ".htm":
		return []string{with(strings.TrimSuffix(p, path.Ext(p)) + ".md")}
	case ext != "":
		return nil // an image, a PDF, a script: no markdown twin
	}
	return []string{with(p + ".md")}
}

// isHTML reports whether a response is a web page.
func isHTML(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mt == "text/html" || mt == "application/xhtml+xml")
}

// markdownVersion asks the server for the .md version of an HTML page and returns it as the page to show,
// marked with the address of the page it stands for. A server that has none (a 404, or an HTML page standing in
// for it) simply gives nothing.
func (b *Browser) markdownVersion(ctx context.Context, res *fetch.Result, on bool) (page.Page, bool) {
	if !on || !isHTML(res.ContentType) {
		return page.Page{}, false
	}
	for _, alt := range markdownURLs(res.URL) {
		got, err := b.fetcher.Get(ctx, alt)
		if err != nil || got.Status != http.StatusOK || !looksMarkdown(got) {
			continue
		}
		base, err := url.Parse(got.URL)
		if err != nil {
			continue
		}
		p := b.markdownPage(page.Page{URL: got.URL, Status: got.Status, ContentType: got.ContentType}, got, base)
		if p.Kind != page.KindMarkdown {
			continue
		}
		p.MarkdownOf = res.URL
		return p, true
	}
	return page.Page{}, false
}

// looksMarkdown is stricter than fetch.IsMarkdown: a server that answers any address with 200 and a text/plain
// page must not pass for one that has markdown.
func looksMarkdown(r *fetch.Result) bool {
	mt, _, err := mime.ParseMediaType(r.ContentType)
	if err != nil {
		return false
	}
	switch mt {
	case "text/markdown", "text/x-markdown":
		return true
	case "text/plain":
		head := strings.TrimSpace(string(r.Body[:min(len(r.Body), 512)]))
		return head != "" && !strings.HasPrefix(head, "<")
	}
	return false
}
