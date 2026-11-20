package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"mardo/internal/fetch"
	"mardo/internal/history"
)

func tocServer(t *testing.T) (*httptest.Server, *int) {
	foreign := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreign++
		w.Write([]byte(`<opml version="2.0"><body><outline text="x" url="http://elsewhere.test/x.md"/></body></opml>`))
	}))
	t.Cleanup(other.Close)
	mux := http.NewServeMux()
	opml := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="ISO-8859-1"?><opml version="2.0"><head><title>t</title></head><body>%s</body></opml>`, body)
		}
	}
	md := func(text string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Write([]byte(text))
		}
	}
	// The site's outline includes the blog's, and one from another site that must not be followed.
	mux.HandleFunc("/.well-known/toc.opml", opml(`<outline text="Blog" type="include" url="/blog/.well-known/toc.opml"/><outline text="Elsewhere" type="include" url="`+other.URL+`/toc.opml"/>`))
	mux.HandleFunc("/blog/.well-known/toc.opml", opml(`<outline text="2026" type="calendarYear"><outline text="November" type="calendarMonth" url="/blog/2026/11.md">`+
		`<outline text="3" type="calendarDay" url="/blog/d3.md"/><outline text="2" type="calendarDay" url="/blog/d2.md"/><outline text="1" type="calendarDay" url="/blog/d1.md"/></outline></outline>`))
	mux.HandleFunc("/blog/d1.md", md("# One\n"))
	mux.HandleFunc("/blog/d2.md", md("# Two\n"))
	mux.HandleFunc("/blog/d3.md", md("# Three\n"))
	mux.HandleFunc("/blog/own.md", md("---\nurlNext: /elsewhere.md\n---\n# Own\n"))
	// A folder with a table of contents of its own.
	mux.HandleFunc("/blog/sub/.well-known/toc.opml", opml(`<outline text="b" url="/blog/sub/b.md"/><outline text="a" url="/blog/sub/a.md"/>`))
	mux.HandleFunc("/blog/sub/a.md", md("# A\n"))
	mux.HandleFunc("/blog/sub/b.md", md("# B\n"))
	mux.HandleFunc("/slow/.well-known/toc.opml", func(w http.ResponseWriter, r *http.Request) { time.Sleep(400 * time.Millisecond) })
	mux.HandleFunc("/slow/p.md", md("# Slow\n"))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &foreign
}

func TestTableOfContentsGivesArrowsInBonbonMode(t *testing.T) {
	srv, foreign := tocServer(t)
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	ctx := context.Background()

	// The outline is newest first: the right arrow is the newer page, the left the older.
	p := b.load(ctx, srv.URL+"/blog/d2.md")
	if !strings.Contains(p.HTML, `class="bonbon-prev" href="`+srv.URL+`/blog/d1.md"`) || !strings.Contains(p.HTML, `class="bonbon-next" href="`+srv.URL+`/blog/d3.md"`) {
		t.Errorf("d2 has d1 on the left and d3 on the right: %s", p.HTML)
	}
	if p := b.load(ctx, srv.URL+"/blog/d3.md"); strings.Contains(p.HTML, "bonbon-next") || !strings.Contains(p.HTML, "bonbon-prev") {
		t.Errorf("the newest page has only a left arrow: %s", p.HTML)
	}
	// A page's own arrows win, and a page the outline does not list has none.
	if p := b.load(ctx, srv.URL+"/blog/own.md"); !strings.Contains(p.HTML, "/elsewhere.md") || strings.Contains(p.HTML, "bonbon-prev") {
		t.Errorf("own arrows: %s", p.HTML)
	}
	// The table of contents of a folder is for the pages under it.
	if p := b.load(ctx, srv.URL+"/blog/sub/a.md"); !strings.Contains(p.HTML, `class="bonbon-next" href="`+srv.URL+`/blog/sub/b.md"`) || strings.Contains(p.HTML, "bonbon-prev") {
		t.Errorf("sub/a: %s", p.HTML)
	}
	if *foreign != 0 {
		t.Errorf("an outline from another site was fetched %d times", *foreign)
	}
	// Outside Bonbon mode nothing is looked up.
	b.SetBonbon(false)
	if p := b.load(ctx, srv.URL+"/blog/d2.md"); strings.Contains(p.HTML, "bonbon-") {
		t.Errorf("no arrows outside Bonbon mode: %s", p.HTML)
	}
}

func TestSlowTableOfContentsDoesNotHoldUpThePage(t *testing.T) {
	srv, _ := tocServer(t)
	old := tocTimeout
	tocTimeout = 100 * time.Millisecond
	defer func() { tocTimeout = old }()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	start := time.Now()
	p := b.load(context.Background(), srv.URL+"/slow/p.md")
	if !strings.Contains(p.HTML, "Slow") || strings.Contains(p.HTML, "bonbon-") || time.Since(start) > 350*time.Millisecond {
		t.Errorf("the page loads without arrows, quickly: %s (%v)", p.HTML, time.Since(start))
	}
}

// In Bonbon mode the words typed on a site that has a search (bonbon.json) also get that search's results, first,
// and the results page remembers it for Reload.
func TestSiteSearchResultsComeFirst(t *testing.T) {
	var asked string
	mux := http.NewServeMux()
	mux.HandleFunc("/blog/.well-known/bonbon.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"urlSearch": "/search", "urlMirrorOf": "http://example.test/"}`))
	})
	mux.HandleFunc("/blog/p.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# A page\n"))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Write([]byte("---\ntitle: Search\n---\n### [A hit](/blog/hit.md)\n"))
	})
	mux.HandleFunc("/other/p.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# No search here\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	b.SetSearcher(func(ctx context.Context, q string) (string, error) {
		return "# Results for “" + q + "”\n\nNothing found in your bookmarks or history.\n", nil
	})
	id := b.NewTab()
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	ctx := context.Background()
	b.Navigate(ctx, id, srv.URL+"/blog/p.md")

	p := b.Navigate(ctx, id, "rust async")
	hit, mine := strings.Index(p.HTML, `href="`+srv.URL+`/blog/hit.md"`), strings.Index(p.HTML, "Nothing found")
	if asked != "rust async" || hit < 0 || mine < 0 || hit > mine || !strings.Contains(p.HTML, `<h2 id="site-results">From 127.0.0.1:`) {
		t.Fatalf("site results come before the bookmarks (asked %q): %s", asked, p.HTML)
	}
	if !strings.Contains(p.URL, "&site=") || p.Title != "Results for “rust async”" {
		t.Errorf("url %q, title %q", p.URL, p.Title)
	}
	asked = ""
	if r := b.Reload(ctx, id); !strings.Contains(r.HTML, "A hit") || asked != "rust async" {
		t.Errorf("Reload keeps the site search: %s", r.HTML)
	}
	// Another search from the results page uses the same site.
	if p := b.Navigate(ctx, id, "go"); !strings.Contains(p.HTML, "A hit") {
		t.Errorf("a new search from the results: %s", p.HTML)
	}
	// A site without a search gets the bookmarks alone, and so does a page outside Bonbon mode.
	b.Navigate(ctx, id, srv.URL+"/other/p.md")
	if p := b.Navigate(ctx, id, "rust"); strings.Contains(p.HTML, "site-results") || !strings.Contains(p.HTML, "Nothing found") {
		t.Errorf("no site search: %s", p.HTML)
	}
}

func TestSiteSearchThatFailsLeavesANote(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/bonbon.json", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"urlSearch": "/search"}`)) })
	mux.HandleFunc("/p.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# P\n"))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusServiceUnavailable) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	b.SetSearcher(func(ctx context.Context, q string) (string, error) { return "# Results for x\n\nNothing found.\n", nil })
	id := b.NewTab()
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	b.Navigate(context.Background(), id, srv.URL+"/p.md")
	p := b.Navigate(context.Background(), id, "x")
	if !strings.Contains(p.HTML, "did not answer: 503 Service Unavailable") || !strings.Contains(p.HTML, "Nothing found") {
		t.Errorf("a note, and the bookmarks: %s", p.HTML)
	}
}

// A results page has links to its sections under the title: the site's, the bookmarks and the history.
func TestResultsPageLinksToItsSections(t *testing.T) {
	srv, _ := siteWithSearch(t)
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	b.SetSearcher(func(ctx context.Context, q string) (string, error) {
		return "# Results for “" + q + "”\n\n1 bookmark, 2 pages from history.\n\n## Bookmarks\n\n- [One](<https://a.test/>)  \n  _Bookmarks_\n\n## History\n\n- [Two](<https://b.test/>)  \n  _History_\n- [Three](<https://c.test/>)  \n  _History_\n", nil
	})
	id := b.NewTab()
	ctx := context.Background()
	// No site search: the two sections the reader's own pages come in.
	if p := b.Navigate(ctx, id, "rust"); !strings.Contains(p.HTML, `<p class="search-jump"><a href="#bookmarks">Bookmarks (1)</a> · <a href="#history">History (2)</a></p>`) || !strings.Contains(p.HTML, `id="history"`) {
		t.Errorf("two links: %s", p.HTML)
	}
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	b.Navigate(ctx, id, srv.URL+"/blog/p.md")
	p := b.Navigate(ctx, id, "rust")
	want := `<p class="search-jump"><a href="#site-results">From ` + strings.TrimPrefix(srv.URL, "http://") + `</a> · <a href="#bookmarks">Bookmarks (1)</a> · <a href="#history">History (2)</a></p>`
	if !strings.Contains(p.HTML, want) || !strings.Contains(p.HTML, `<h2 id="site-results">`) || !strings.Contains(p.HTML, `id="bookmarks"`) {
		t.Errorf("three links: %s", p.HTML)
	}
}

func siteWithSearch(t *testing.T) (*httptest.Server, *string) {
	asked := ""
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/bonbon.json", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"urlSearch": "/search"}`)) })
	mux.HandleFunc("/blog/p.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# P\n"))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("### A hit\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &asked
}

// With a site's results and nothing of the reader's own, the line still names them, as text.
func TestResultsLineNamesEmptySectionsWhenASiteAnswers(t *testing.T) {
	srv, _ := siteWithSearch(t)
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	b.SetSearcher(func(ctx context.Context, q string) (string, error) {
		return "# Results for “" + q + "”\n\nNothing found in your bookmarks or history.\n", nil
	})
	id := b.NewTab()
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	b.Navigate(context.Background(), id, srv.URL+"/blog/p.md")
	p := b.Navigate(context.Background(), id, "trump")
	if !strings.Contains(p.HTML, `<a href="#site-results">From `) || !strings.Contains(p.HTML, "<span>Bookmarks (0)</span> · <span>History (0)</span>") {
		t.Errorf("%s", p.HTML)
	}
}

// A click on a link to the HTML site this one mirrors (urlMirrorOf) goes to the same page here, if there is one.
func TestLinkToTheMirroredSiteGoesToItsMarkdownTwin(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/blog/.well-known/bonbon.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"urlMirrorOf": "http://html.test/"}`))
	})
	md := func(text string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/markdown")
			w.Write([]byte(text))
		}
	}
	mux.HandleFunc("/blog/p.md", md("# P\n"))
	mux.HandleFunc("/blog/2026/11/18.md", md("# The 18th\n"))
	mux.HandleFunc("/blog/about/index.md", md("# About\n"))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	id := b.NewTab()
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	ctx := context.Background()
	b.Follow(ctx, id, srv.URL+"/blog/p.md")
	for link, want := range map[string]string{
		"http://html.test/2026/11/18.html#a1": "/blog/2026/11/18.md",
		"http://html.test/about/":             "/blog/about/index.md",
		"http://html.test/about":              "/blog/about/index.md",
		"http://HTML.test/2026/11/18.html":    "/blog/2026/11/18.md",
		"http://html.test/missing.html":       "",
		"http://html.test/pic.png":            "",
		"http://other.test/2026/11/18.html":   "",
		"http://html.test.evil.test/about/":   "",
	} {
		got := b.mirrorOf(ctx, srv.URL+"/blog/p.md", link)
		if want != "" {
			want = srv.URL + want
		}
		if got != want {
			t.Errorf("%s -> %q; want %q", link, got, want)
		}
	}
	p := b.Follow(ctx, id, "http://html.test/2026/11/18.html")
	if p.Kind != "markdown" || p.URL != srv.URL+"/blog/2026/11/18.md" || !strings.Contains(p.HTML, "The 18th") {
		t.Errorf("a click opens the twin: %+v", p)
	}
}

// A site that is read at another address (a repository's) and asks for a name of its own (cname) gets an offer of it.
func TestSiteThatAsksForAnAddressOfItsOwn(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/user/repo/main/.well-known/bonbon.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"cname": "Demo.Example.org"}`))
	})
	mux.HandleFunc("/user/repo/main/docs/readme.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# Readme\n"))
	})
	mux.HandleFunc("/user/other/readme.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# Other\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	ctx := context.Background()
	if p := b.load(ctx, srv.URL+"/user/repo/main/docs/readme.md"); p.OwnAddress != "https://demo.example.org/docs/readme.md" {
		t.Errorf("own address = %q", p.OwnAddress)
	}
	if p := b.load(ctx, srv.URL+"/user/other/readme.md"); p.OwnAddress != "" {
		t.Errorf("a page outside the site has none: %q", p.OwnAddress)
	}
	c := &SiteConfig{CName: "not a host", url: "https://x.test/.well-known/bonbon.json"}
	u, _ := url.Parse("https://x.test/a.md")
	if got := c.ownAddress(u); got != "" {
		t.Errorf("an invalid name: %q", got)
	}
	c.CName = "x.test"
	if got := c.ownAddress(u); got != "" {
		t.Errorf("the site is already there: %q", got)
	}
}
