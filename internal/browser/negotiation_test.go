package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"mardo/internal/fetch"
	"mardo/internal/history"
	"mardo/internal/page"
)

// prefersMarkdown is how a server should choose by the Accept header: markdown only when it is named, by the
// q-values, a tie going to markdown; a wildcard alone is the web page.
func prefersMarkdown(accept string) bool {
	md, html := 0.0, 0.0
	for _, part := range strings.Split(accept, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		q := 1.0
		for _, f := range fields[1:] {
			if k, v, ok := strings.Cut(strings.TrimSpace(f), "="); ok && k == "q" {
				q, _ = strconv.ParseFloat(v, 64)
			}
		}
		switch strings.ToLower(strings.TrimSpace(fields[0])) {
		case "text/markdown", "text/x-markdown":
			md = max(md, q)
		case "text/html", "application/xhtml+xml", "text/*", "*/*":
			html = max(html, q)
		}
	}
	return md > 0 && md >= html
}

func TestPrefersMarkdown(t *testing.T) {
	for accept, want := range map[string]bool{
		"*/*": false,
		"text/html,application/xhtml+xml,*/*;q=0.8": false,
		"text/markdown":                  true,
		"text/markdown, text/html;q=0.9": true,
		"text/html, text/markdown;q=0.5": false,
		"text/markdown, text/html":       true, // tied
		fetch.Accept:                     true,
		fetch.AcceptHTML:                 false,
		"":                               false,
	} {
		if got := prefersMarkdown(accept); got != want {
			t.Errorf("prefersMarkdown(%q) = %v; want %v", accept, got, want)
		}
	}
}

// negotiatingServer has /post answer by Accept, with the headers that say so; /header and /static have their
// markdown elsewhere, named in a Link header; /static.md is sent as a download.
func negotiatingServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Vary", "Accept")
		if prefersMarkdown(r.Header.Get("Accept")) {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Header().Set("Content-Location", "/post.md")
			w.Write([]byte("# The post, in markdown\n"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>The post</title></head><body><a href="/x">the web page</a></body></html>`))
	})
	mux.HandleFunc("/post.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Vary", "Accept")
		w.Write([]byte("# The post, in markdown\n"))
	})
	twin := func(w http.ResponseWriter, link string) {
		w.Header().Set("Link", link)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Web only</title></head><body><a href="/x">x</a></body></html>`))
	}
	mux.HandleFunc("/header", func(w http.ResponseWriter, r *http.Request) {
		twin(w, `</elsewhere/header.md>; rel="alternate"; type="text/markdown"`)
	})
	mux.HandleFunc("/quoted", func(w http.ResponseWriter, r *http.Request) {
		twin(w, `</rss>; rel="alternate"; type="application/rss+xml", </elsewhere/header.md>; type="text/markdown; charset=utf-8"; rel="alternate"; title="x, y"`)
	})
	mux.HandleFunc("/elsewhere/header.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# From the Link header\n"))
	})
	mux.HandleFunc("/static", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Static</title></head></html>`))
	})
	mux.HandleFunc("/static.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("# Static, sent as a download\n"))
	})
	mux.HandleFunc("/blob", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Blob</title></head></html>`))
	})
	mux.HandleFunc("/blob.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte{0x89, 'P', 'N', 'G', 0, 1, 2, 3})
	})
	return httptest.NewServer(mux)
}

func TestSameAddressAnswersWithMarkdownAndTheWebPage(t *testing.T) {
	srv := negotiatingServer()
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	ctx := context.Background()

	p := b.load(ctx, srv.URL+"/post")
	if p.Kind != page.KindMarkdown || p.Title != "The post, in markdown" {
		t.Fatalf("page = kind %v title %q error %q", p.Kind, p.Title, p.Error)
	}
	if !p.Negotiated || p.MarkdownOf != "" {
		t.Errorf("negotiated = %v, markdownOf = %q; want a negotiated page that is not a twin", p.Negotiated, p.MarkdownOf)
	}

	// Seeing the web page asks for the web page, at the same address.
	tab := b.NewTab()
	web := b.NavigateAsIs(ctx, tab, srv.URL+"/post")
	if web.Kind != page.KindFallback || web.Title != "The post" || web.Negotiated {
		t.Errorf("as is = kind %v title %q negotiated %v; want the web page", web.Kind, web.Title, web.Negotiated)
	}

	// A markdown file at its own address is not negotiated.
	if file := b.load(ctx, srv.URL+"/post.md"); file.Kind != page.KindMarkdown || file.Negotiated {
		t.Errorf("/post.md = kind %v negotiated %v", file.Kind, file.Negotiated)
	}
}

func TestMarkdownNamedInALinkHeader(t *testing.T) {
	srv := negotiatingServer()
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	for _, path := range []string{"/header", "/quoted"} {
		p := b.load(context.Background(), srv.URL+path)
		if p.Kind != page.KindMarkdown || p.Title != "From the Link header" || p.MarkdownOf != srv.URL+path {
			t.Errorf("%s = kind %v title %q markdownOf %q error %q", path, p.Kind, p.Title, p.MarkdownOf, p.Error)
		}
	}
	// With the lookup off, the page stays the web page.
	b.SetMarkdownVersions(false)
	if p := b.load(context.Background(), srv.URL+"/header"); p.Kind != page.KindFallback {
		t.Errorf("lookup off: kind %v; want the fallback page", p.Kind)
	}
}

func TestMarkdownFileSentAsADownload(t *testing.T) {
	srv := negotiatingServer()
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	if p := b.load(context.Background(), srv.URL+"/static"); p.Kind != page.KindMarkdown || p.Title != "Static, sent as a download" {
		t.Errorf("/static = kind %v title %q", p.Kind, p.Title)
	}
	// Bytes that are not text are not markdown, whatever the address says.
	if p := b.load(context.Background(), srv.URL+"/blob"); p.Kind != page.KindFallback {
		t.Errorf("/blob = kind %v; want the fallback page", p.Kind)
	}
}

func TestKeepTextFragment(t *testing.T) {
	asked := "https://a.test/post#:~:text=hello%20there"
	if got := keepTextFragment(asked, page.Page{Kind: page.KindMarkdown, URL: "https://a.test/post.md"}); got.URL != asked[:20]+".md#:~:text=hello%20there" && got.URL != "https://a.test/post.md#:~:text=hello%20there" {
		t.Errorf("fragment not kept: %q", got.URL)
	}
	if got := keepTextFragment(asked, page.Page{Kind: page.KindMarkdown, URL: asked}); got.URL != asked {
		t.Errorf("fragment doubled: %q", got.URL)
	}
	if got := keepTextFragment("https://a.test/post#top", page.Page{Kind: page.KindMarkdown, URL: "https://a.test/post"}); got.URL != "https://a.test/post" {
		t.Errorf("an ordinary fragment was added: %q", got.URL)
	}
}

// In Bonbon mode a request asks the Bonbon way, a web page is shown as its source and not looked past, and
// only an address ending in .md makes a text/plain answer a page.
func TestBonbonMode(t *testing.T) {
	var accept string
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>T</title><link rel="alternate" type="text/markdown" href="/page.md"></head><body><b>hi</b></body></html>`))
	})
	mux.HandleFunc("/page.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# From a .md file <b>x</b>\n"))
	})
	mux.HandleFunc("/notes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# Not a page\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	b := New(history.Open(t.TempDir()+"/h.json"), fetch.NewFetcher(nil, nil))
	b.SetBonbon(true)
	defer b.SetBonbon(false)

	p := b.load(context.Background(), srv.URL+"/page")
	if accept != fetch.AcceptBonbon {
		t.Errorf("Accept = %q; want %q", accept, fetch.AcceptBonbon)
	}
	if p.Kind != page.KindMarkdown || p.MarkdownOf != "" || !strings.Contains(p.HTML, "&lt;b&gt;hi&lt;/b&gt;") || strings.Contains(p.HTML, "<b>") {
		t.Errorf("a web page is shown as text: kind %q, markdownOf %q, html %q", p.Kind, p.MarkdownOf, p.HTML)
	}
	if p := b.load(context.Background(), srv.URL+"/page.md"); p.Kind != page.KindMarkdown || strings.Contains(p.HTML, "<b>") || !strings.Contains(p.HTML, "From a .md file") {
		t.Errorf("a .md file is a page, without its HTML: %q", p.HTML)
	}
	if p := b.load(context.Background(), srv.URL+"/notes"); !strings.Contains(p.HTML, "<pre>") {
		t.Errorf("text/plain without .md is shown as text: %q", p.HTML)
	}
}

// A clicked link to a web page is for the default browser in Bonbon mode: the answer says so and the tab does not move.
func TestBonbonFollowLeavesWebPagesToTheBrowser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/web", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body>hi</body></html>"))
	})
	mux.HandleFunc("/doc.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# Doc\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	id := b.NewTab()
	b.SetBonbon(true)
	defer b.SetBonbon(false)
	first := b.Follow(context.Background(), id, srv.URL+"/doc.md")
	if first.Kind != page.KindMarkdown {
		t.Fatalf("a markdown link loads here: %+v", first)
	}
	if p := b.Follow(context.Background(), id, srv.URL+"/web"); p.Kind != page.KindExternal || p.URL != srv.URL+"/web" {
		t.Errorf("a web page is for the browser: %+v", p)
	}
	if p := b.Reload(context.Background(), id); p.URL != first.URL {
		t.Errorf("the tab stayed on %q, but is on %q", first.URL, p.URL)
	}
}

func TestDataAddressIsAPage(t *testing.T) {
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	for name, addr := range map[string]string{
		"base64":  "data:text/markdown;charset=utf-8;base64,IyBIZWxsbywgV29ybGQhCgpUaGlzIGlzIGEgc2ltcGxlIE1hcmtkb3duIGZpbGUuCg==",
		"percent": "data:text/markdown,%23%20Hello%2C%20World!%0A%0AThis is a simple Markdown file.",
		"no type": "data:,%23%20Hello%2C%20World!",
	} {
		p := b.Navigate(context.Background(), 0, addr)
		if p.Kind == page.KindError || p.Kind != page.KindMarkdown || !strings.Contains(p.HTML, "Hello, World!") || !strings.HasPrefix(p.URL, "data:") {
			t.Errorf("%s: kind %q url %.20q error %q html %q", name, p.Kind, p.URL, p.Error, p.HTML)
		}
	}
	if got := b.History(); len(got) != 0 {
		t.Errorf("a data page was put in the history: %v", got)
	}
}
