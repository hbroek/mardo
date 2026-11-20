package browser

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"mardo/internal/fetch"
	"mardo/internal/page"
	"mardo/internal/render"
)

// searchTimeout is how long a site's search may take before the results page is shown without it. A variable so a
// test can shorten it.
var searchTimeout = 10 * time.Second

// withSiteResults makes the results page p, the bookmark and history search for query, also show what the search
// of the site at endpoint finds, ahead of the bookmarks. Words and addresses are the reader's own: the site hears
// the words, so the page says whose search it is. A search that does not answer leaves a note in its place.
func (b *Browser) withSiteResults(ctx context.Context, p page.Page, endpoint, query, bookmarksMD string, base *url.URL) page.Page {
	host := endpoint
	if u, err := url.Parse(endpoint); err == nil {
		host = u.Host
	}
	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()
	siteHTML, siteMD := b.siteResults(ctx, endpoint, query, host)

	// The page has one title; the bookmark results come under their own heading.
	if strings.HasPrefix(bookmarksMD, "# ") {
		if i := strings.Index(bookmarksMD, "\n\n"); i >= 0 {
			bookmarksMD = bookmarksMD[i+2:]
		}
	}
	bookmarksHTML, _, err := render.Markdown([]byte(bookmarksMD), base)
	if err != nil {
		return p
	}
	title := "Results for “" + query + "”"
	e := html.EscapeString
	nav := jumpLinks([]jump{{"From " + host, "site-results", -1}}, bookmarksMD, true)
	p.HTML = "<h1>" + e(title) + "</h1>" + nav + `<h2 id="site-results">From ` + e(host) + "</h2>" + siteHTML + bookmarksHTML
	p.Source = "# " + title + "\n\n## From " + host + "\n\n" + siteMD + "\n\n" + bookmarksMD
	p.Title = title
	return p
}

// siteResults asks the search at endpoint for query and renders the markdown page it sends back.
func (b *Browser) siteResults(ctx context.Context, endpoint, query, host string) (htmlOut, md string) {
	fail := func(why string) (string, string) {
		note := fmt.Sprintf("The search on %s did not answer: %s.", host, why)
		return "<p><em>" + html.EscapeString(note) + "</em></p>", note
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return fail("its address is not valid")
	}
	q := u.Query()
	q.Set("q", query)
	u.RawQuery = q.Encode()
	res, err := b.fetcher.Get(ctx, u.String())
	switch {
	case err != nil:
		return fail(err.Error())
	case res.Status != http.StatusOK:
		return fail(fmt.Sprintf("%d %s", res.Status, http.StatusText(res.Status)))
	case !fetch.IsBonbonMarkdown(res.ContentType, u.Path):
		return fail("it did not send markdown")
	}
	base, err := url.Parse(res.URL)
	if err != nil {
		return fail("its address is not valid")
	}
	out, _, err := render.Markdown(res.Body, base)
	if err != nil {
		return fail(err.Error())
	}
	return out, string(res.Body)
}

// jump is a link to a section of a results page: its words, the id of its heading and how many results it holds
// (-1: not shown).
type jump struct {
	label, id string
	n         int
}

// jumpLinks is the line of links under the title of a results page, to the sections that are there: the first
// ones given (the site's results), then the bookmarks and the history of md, the markdown of the bookmark
// search. With all, the bookmarks and the history are named even when they have no results (not as links).
// "" when there are fewer than two, since one section needs no way to it.
func jumpLinks(first []jump, md string, all bool) string {
	jumps := append([]jump(nil), first...)
	section, counts := "", map[string]int{}
	var order []string
	for _, line := range strings.Split(md, "\n") {
		switch {
		case line == "## Bookmarks":
			section = "bookmarks"
			order = append(order, section)
		case line == "## History":
			section = "history"
			order = append(order, section)
		case section != "" && strings.HasPrefix(line, "- ["):
			counts[section]++
		}
	}
	for _, id := range order {
		jumps = append(jumps, jump{strings.ToUpper(id[:1]) + id[1:], id, counts[id]})
	}
	if all { // with a site's results the line always names both of the reader's own, as plain text when empty
		for _, id := range []string{"bookmarks", "history"} {
			if _, ok := counts[id]; !ok {
				jumps = append(jumps, jump{strings.ToUpper(id[:1]) + id[1:], "", 0})
			}
		}
	}
	if len(jumps) < 2 {
		return ""
	}
	links := make([]string, len(jumps))
	for i, j := range jumps {
		label := html.EscapeString(j.label)
		if j.n >= 0 {
			label += fmt.Sprintf(" (%d)", j.n)
		}
		if j.id == "" {
			links[i] = "<span>" + label + "</span>"
			continue
		}
		links[i] = `<a href="#` + j.id + `">` + label + `</a>`
	}
	return `<p class="search-jump">` + strings.Join(links, " \u00B7 ") + `</p>`
}
