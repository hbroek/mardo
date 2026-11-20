package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
