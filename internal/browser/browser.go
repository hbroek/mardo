// Package browser ties fetching, rendering and navigation together.
package browser

import (
	"context"
	"fmt"
	"html"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"mardo/internal/auth"
	"mardo/internal/fetch"
	"mardo/internal/history"
	"mardo/internal/nav"
	"mardo/internal/page"
	"mardo/internal/render"
)

// Searcher turns a bookmark search query into a markdown page.
type Searcher func(ctx context.Context, query string) (string, error)

// searchPrefix starts the pseudo-URL of a bookmark search page, so Back,
// Forward and Reload can rebuild it: "bookmarks:search?q=rust+async".
const searchPrefix = "bookmarks:search?q="

// SearchURL is the address of the results page for query.
func SearchURL(query string) string { return searchPrefix + url.QueryEscape(query) }

// meaningPrefix starts the pseudo-URL of a search by meaning ("@ ..." in the address bar).
const meaningPrefix = "bookmarks:meaning?q="

// MeaningURL is the address of the results page of a search by meaning.
func MeaningURL(query string) string { return meaningPrefix + url.QueryEscape(query) }

func isSearchURL(u string) bool {
	return strings.HasPrefix(u, searchPrefix) || strings.HasPrefix(u, meaningPrefix)
}

// tab is one page with its own back/forward history. Its lock is held while a
// page loads, so a slow site only holds up its own tab.
type tab struct {
	mu    sync.Mutex
	stack *nav.Stack
}

type Browser struct {
	fetcher *fetch.Fetcher
	search  Searcher
	meaning Searcher // searches by meaning, with an AI model
	history *history.Log

	noVariants atomic.Bool // do not look for .md versions of pages (see variant.go)
	bonbon     atomic.Bool // Bonbon mode: markdown only, no looking further (see SetBonbon)
	toc        tocCache    // the tables of contents of sites, for the arrows of pages (see toc.go)

	tabMu   sync.Mutex
	tabs    map[int]*tab
	nextTab int
}

func New(log *history.Log, fetcher *fetch.Fetcher) *Browser {
	return &Browser{history: log, fetcher: fetcher, tabs: map[int]*tab{}}
}

// NewTab opens an empty tab and returns its id. The history is shared by all tabs.
func (b *Browser) NewTab() int {
	b.tabMu.Lock()
	defer b.tabMu.Unlock()
	b.nextTab++
	b.tabs[b.nextTab] = &tab{stack: nav.NewStack()}
	return b.nextTab
}

// CloseTab forgets the tab's back/forward history.
func (b *Browser) CloseTab(id int) {
	b.tabMu.Lock()
	defer b.tabMu.Unlock()
	delete(b.tabs, id)
}

// tab returns the tab with this id, creating it if it is not known yet.
func (b *Browser) tab(id int) *tab {
	b.tabMu.Lock()
	defer b.tabMu.Unlock()
	t, ok := b.tabs[id]
	if !ok {
		t = &tab{stack: nav.NewStack()}
		b.tabs[id] = t
	}
	return t
}

// SetSearcher enables bookmark searches typed into the address bar.
func (b *Browser) SetSearcher(s Searcher) { b.search = s }

// SetMeaningSearcher enables searches by meaning, typed as "@ ..." in the address bar.
func (b *Browser) SetMeaningSearcher(s Searcher) { b.meaning = s }

// History returns the visit log, most recent first.
func (b *Browser) History() []history.Entry { return b.history.Entries() }

func (b *Browser) ClearHistory() { b.history.Clear() }

// FullHistory writes the visit log as a markdown file and opens it.
func (b *Browser) FullHistory(ctx context.Context, tabID int) page.Page {
	path, err := b.history.WriteMarkdown()
	if err != nil {
		t := b.tab(tabID)
		t.mu.Lock()
		defer t.mu.Unlock()
		return t.withState(page.Page{Kind: page.KindError, Error: "cannot write history: " + err.Error()})
	}
	return b.Navigate(ctx, tabID, path)
}

// record adds a successfully loaded page to the visit log.
func (b *Browser) record(p page.Page) page.Page {
	if p.Kind != page.KindError && !isSearchURL(p.URL) { // searches are not visits
		b.history.Add(p.URL, p.Title)
	}
	return p
}

// Navigate loads rawURL and makes it the current entry.
func (b *Browser) Navigate(ctx context.Context, tabID int, rawURL string) page.Page {
	t := b.tab(tabID)
	t.mu.Lock()
	defer t.mu.Unlock()
	if q, ok := fetch.MeaningQuery(rawURL); ok {
		rawURL = MeaningURL(q)
	} else if q, ok := fetch.SearchQuery(rawURL); ok {
		rawURL = SearchURL(q)
	}
	p := keepTextFragment(rawURL, b.load(ctx, rawURL))
	t.stack.Visit(p.URL)
	return t.withState(b.record(p))
}

// Follow opens rawURL for a click on a link. Outside Bonbon mode it is Navigate. In it a link to a web page
// (HTML) is not shown here: the answer is KindExternal, for the reader's default browser, and the tab and its
// history stay as they are. Anything else loads as usual.
func (b *Browser) Follow(ctx context.Context, tabID int, rawURL string) page.Page {
	if !b.Bonbon() {
		return b.Navigate(ctx, tabID, rawURL)
	}
	t := b.tab(tabID)
	t.mu.Lock()
	defer t.mu.Unlock()
	p := keepTextFragment(rawURL, b.load(ctx, rawURL))
	if isHTML(p.ContentType) {
		return page.Page{URL: p.URL, Kind: page.KindExternal}
	}
	t.stack.Visit(p.URL)
	return t.withState(b.record(p))
}

// keepTextFragment keeps the "#:~:text=" part of the address asked for, which points at a passage,
// when the page that came back (a .md twin, say) is at an address without it.
func keepTextFragment(asked string, p page.Page) page.Page {
	i := strings.Index(asked, "#:~:text=")
	if i < 0 || p.URL == "" || strings.Contains(p.URL, "#") || p.Kind == page.KindError {
		return p
	}
	p.URL += asked[i:]
	return p
}

// Reload fetches the current entry again.
func (b *Browser) Reload(ctx context.Context, tabID int) page.Page {
	t := b.tab(tabID)
	t.mu.Lock()
	defer t.mu.Unlock()
	cur := t.stack.Current()
	if cur == "" {
		return t.withState(page.Page{Kind: page.KindError, Error: "nothing to reload"})
	}
	return t.withState(b.load(ctx, cur))
}

// Back loads the previous entry.
func (b *Browser) Back(ctx context.Context, tabID int) page.Page {
	t := b.tab(tabID)
	t.mu.Lock()
	defer t.mu.Unlock()
	u, ok := t.stack.Back()
	if !ok {
		return t.withState(page.Page{URL: t.stack.Current(), Kind: page.KindError, Error: "no earlier page"})
	}
	return t.withState(b.record(b.load(ctx, u)))
}

// Forward loads the next entry.
func (b *Browser) Forward(ctx context.Context, tabID int) page.Page {
	t := b.tab(tabID)
	t.mu.Lock()
	defer t.mu.Unlock()
	u, ok := t.stack.Forward()
	if !ok {
		return t.withState(page.Page{URL: t.stack.Current(), Kind: page.KindError, Error: "no later page"})
	}
	return t.withState(b.record(b.load(ctx, u)))
}

func (t *tab) withState(p page.Page) page.Page {
	p.CanBack = t.stack.CanBack()
	p.CanForward = t.stack.CanForward()
	return p
}

// SetMarkdownVersions turns on or off looking for a .md version of web pages that are not markdown.
func (b *Browser) SetMarkdownVersions(on bool) { b.noVariants.Store(!on) }

// SetBonbon turns Bonbon mode on or off. In it a page is markdown or it is shown as text: requests ask the way
// Bonbon does, a web page, feed or other file is not converted or looked past for a markdown version, and the
// markdown is rendered without HTML, SVG pictures and the extra page layout (see render.Bonbon).
func (b *Browser) SetBonbon(on bool) {
	b.bonbon.Store(on)
	b.fetcher.SetBonbon(on)
	render.Bonbon.Store(on)
}

// Bonbon says whether Bonbon mode is on.
func (b *Browser) Bonbon() bool { return b.bonbon.Load() }

// MarkdownVersions says whether .md versions are looked for.
func (b *Browser) MarkdownVersions() bool { return !b.noVariants.Load() }

// NavigateAsIs loads rawURL without looking for a .md version, so a page that has one can still be seen as it is.
func (b *Browser) NavigateAsIs(ctx context.Context, tabID int, rawURL string) page.Page {
	t := b.tab(tabID)
	t.mu.Lock()
	defer t.mu.Unlock()
	p := keepTextFragment(rawURL, b.loadPage(ctx, rawURL, false))
	t.stack.Visit(p.URL)
	return t.withState(b.record(p))
}

// load fetches rawURL and builds the Page to display. Failures become error pages.
func (b *Browser) load(ctx context.Context, rawURL string) page.Page {
	return b.loadPage(ctx, rawURL, b.MarkdownVersions() && !b.Bonbon())
}

func (b *Browser) loadPage(ctx context.Context, rawURL string, versions bool) page.Page {
	if isSearchURL(rawURL) {
		return b.loadSearch(ctx, rawURL)
	}
	// Seeing a page as it is means the web page, also from an address that would answer with markdown.
	get := b.fetcher.Get
	if !versions && !b.Bonbon() {
		get = b.fetcher.GetHTML
	}
	res, err := get(ctx, rawURL)
	if err != nil {
		p := page.Page{URL: rawURL, Kind: page.KindError, Error: err.Error()}
		if u, nerr := fetch.Normalize(rawURL); nerr == nil {
			p.URL = u.String()
		}
		return p
	}
	p := page.Page{URL: res.URL, Status: res.Status, ContentType: res.ContentType}
	if res.Status >= 400 {
		p.Kind = page.KindError
		p.Error = fmt.Sprintf("%d %s", res.Status, http.StatusText(res.Status))
		if res.Status == http.StatusUnauthorized || res.Status == http.StatusForbidden {
			if u, err := url.Parse(res.URL); err == nil {
				p.Origin = auth.Origin(u)
			}
			if f := strings.Fields(res.WWWAuthenticate); len(f) > 0 {
				p.AuthScheme = strings.ToLower(f[0])
			}
		}
		return p
	}
	base, err := url.Parse(res.URL)
	if err != nil {
		p.Kind, p.Error = page.KindError, err.Error()
		return p
	}
	if b.Bonbon() {
		return b.bonbonPage(ctx, p, res, base)
	}
	if render.IsFeed(res.Body) {
		if html, title, ok, err := render.Feed(res.Body, base); err != nil {
			p.Kind, p.Error = page.KindError, err.Error()
			return p
		} else if ok {
			p.Kind, p.HTML, p.Source, p.Title = page.KindMarkdown, html, string(res.Body), title
			if p.Title == "" {
				p.Title = res.URL
			}
			return p
		}
	}
	if fetch.IsMarkdown(res.ContentType) {
		p = b.markdownPage(p, res, base)
		p.Negotiated = versions && fetch.Negotiated(res)
		return p
	} else if alt, ok := b.markdownVersion(ctx, res, versions); ok {
		return alt
	} else {
		p.Kind = page.KindFallback
		p.Title, p.Links = render.Fallback(res.Body, res.ContentType, base)
	}
	if p.Title == "" {
		p.Title = res.URL
	}
	return p
}

// markdownPage fills p from a markdown response.
func (b *Browser) markdownPage(p page.Page, res *fetch.Result, base *url.URL) page.Page {
	return b.markdownPageWith(p, res, base, nil)
}

// markdownPageWith is markdownPage with front matter values (urlprev, urlnext) for a page that has none.
func (b *Browser) markdownPageWith(p page.Page, res *fetch.Result, base *url.URL, defaults map[string]string) page.Page {
	html, title, err := render.MarkdownWith(res.Body, base, defaults)
	if err != nil {
		p.Kind, p.Error = page.KindError, err.Error()
		return p
	}
	p.Kind, p.HTML, p.Source = page.KindMarkdown, html, string(res.Body)
	p.Title = title
	if p.EditURL = fetch.Editable(res); p.EditURL != "" {
		p.ETag = res.Header.Get("ETag")
	}
	if p.Title == "" {
		p.Title = res.URL
	}
	return p
}

// loadSearch builds the bookmark search results page, by keywords or by meaning.
func (b *Browser) loadSearch(ctx context.Context, rawURL string) page.Page {
	p := page.Page{URL: rawURL}
	search, prefix := b.search, searchPrefix
	if strings.HasPrefix(rawURL, meaningPrefix) {
		search, prefix = b.meaning, meaningPrefix
	}
	query, err := url.QueryUnescape(strings.TrimPrefix(rawURL, prefix))
	if err == nil && search == nil {
		err = fmt.Errorf("bookmarks are not available")
	}
	var md string
	if err == nil {
		md, err = search(ctx, query)
	}
	if err != nil {
		p.Kind, p.Error = page.KindError, err.Error()
		return p
	}
	base, _ := url.Parse(rawURL)
	html, title, err := render.Markdown([]byte(md), base)
	if err != nil {
		p.Kind, p.Error = page.KindError, err.Error()
		return p
	}
	p.Kind, p.HTML, p.Source, p.Title = page.KindMarkdown, html, md, title
	return p
}

// bonbonPage builds the page for a response in Bonbon mode: markdown is a page, text of any other kind (HTML,
// XML, OPML, JSON, code) is shown as it is, in color, and the rest (a picture, a download) is left to the
// fallback page.
func (b *Browser) bonbonPage(ctx context.Context, p page.Page, res *fetch.Result, base *url.URL) page.Page {
	if fetch.IsBonbonMarkdown(res.ContentType, base.Path) {
		// A page that names no arrows of its own gets them from the site's table of contents.
		var arrows map[string]string
		if !render.HasArrows(res.Body) {
			arrows = b.tocArrows(ctx, res, base)
		}
		return b.markdownPageWith(p, res, base, arrows)
	}
	if looksText(res) {
		p.Kind, p.Title = page.KindMarkdown, res.URL
		p.Source = string(res.Body)
		p.HTML = "<pre><code>" + html.EscapeString(p.Source) + "</code></pre>"
		return p
	}
	p.Kind = page.KindFallback
	p.Title, p.Links = render.Fallback(res.Body, res.ContentType, base)
	if p.Title == "" {
		p.Title = res.URL
	}
	return p
}

// looksText reports whether a response is text to show as it is: a text type, XML, JSON or JavaScript.
func looksText(r *fetch.Result) bool {
	mt, _, err := mime.ParseMediaType(r.ContentType)
	if err != nil {
		return false
	}
	if strings.HasPrefix(mt, "text/") || strings.HasSuffix(mt, "+xml") || strings.HasSuffix(mt, "+json") {
		return true
	}
	switch mt {
	case "application/xml", "application/json", "application/javascript", "application/x-javascript", "application/x-opml":
		return true
	}
	return false
}
