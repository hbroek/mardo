package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mardo/internal/fetch"
	"mardo/internal/history"
	"mardo/internal/page"
)

func TestMarkdownURLs(t *testing.T) {
	for in, want := range map[string][]string{
		"https://x.test/about":       {"https://x.test/about.md"},
		"https://x.test/docs/":       {"https://x.test/docs/index.md"},
		"https://x.test":             {"https://x.test/index.md", "https://x.test/llms.txt"},
		"https://x.test/":            {"https://x.test/index.md", "https://x.test/llms.txt"},
		"https://x.test/guide.html":  {"https://x.test/guide.md"},
		"https://x.test/a/b#section": {"https://x.test/a/b.md"},
		"https://x.test/a.md":        nil,
		"https://x.test/logo.png":    nil,
		"https://x.test/search?q=go": nil,
		"file:///tmp/a":              nil,
	} {
		got := markdownURLs(in)
		if len(got) != len(want) {
			t.Errorf("markdownURLs(%q) = %v; want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("markdownURLs(%q) = %v; want %v", in, got, want)
			}
		}
	}
}

func variantServer() *httptest.Server {
	mux := http.NewServeMux()
	html := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(body))
	}
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) { html(w, `<title>About</title><a href="/x">x</a>`) })
	mux.HandleFunc("/about.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# About, in markdown\n"))
	})
	mux.HandleFunc("/soft", func(w http.ResponseWriter, r *http.Request) { html(w, "<title>Soft</title>") })
	mux.HandleFunc("/soft.md", func(w http.ResponseWriter, r *http.Request) { html(w, "<h1>Not found, politely</h1>") }) // 200 with HTML
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		html(w, "<title>Home</title>")
	})
	mux.HandleFunc("/llms.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("# Example site\n\n> What the site is.\n"))
	})
	mux.HandleFunc("/plain", func(w http.ResponseWriter, r *http.Request) { html(w, "<title>Plain</title>") })
	mux.HandleFunc("/plain.md", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("# From plain text\n"))
	})
	return httptest.NewServer(mux)
}

func TestHTMLPageWithMarkdownVersionShowsIt(t *testing.T) {
	srv := variantServer()
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	p := b.Navigate(t.Context(), b.NewTab(), srv.URL+"/about")
	if p.Kind != page.KindMarkdown || p.URL != srv.URL+"/about.md" || p.MarkdownOf != srv.URL+"/about" || p.Title != "About, in markdown" {
		t.Fatalf("page = %+v", p)
	}
	// Plain text that is not HTML counts too; an HTML page standing in for a missing .md does not.
	if p := b.Navigate(t.Context(), b.NewTab(), srv.URL+"/plain"); p.Kind != page.KindMarkdown {
		t.Errorf("plain-text version: kind %v", p.Kind)
	}
	if p := b.Navigate(t.Context(), b.NewTab(), srv.URL+"/soft"); p.Kind != page.KindFallback || p.MarkdownOf != "" {
		t.Errorf("soft 404: %+v", p)
	}
}

func TestSiteRootFallsBackToLlmsTxt(t *testing.T) {
	srv := variantServer()
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	p := b.Navigate(t.Context(), b.NewTab(), srv.URL+"/")
	if p.Kind != page.KindMarkdown || p.URL != srv.URL+"/llms.txt" || p.MarkdownOf != srv.URL+"/" || p.Title != "Example site" {
		t.Fatalf("page = %+v", p)
	}
}

func TestMarkdownVersionOffOrAsIs(t *testing.T) {
	srv := variantServer()
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	b.SetMarkdownVersions(false)
	if p := b.Navigate(t.Context(), b.NewTab(), srv.URL+"/about"); p.Kind != page.KindFallback {
		t.Errorf("with the option off: kind %v", p.Kind)
	}
	b.SetMarkdownVersions(true)
	if p := b.NavigateAsIs(t.Context(), b.NewTab(), srv.URL+"/about"); p.Kind != page.KindFallback || p.URL != srv.URL+"/about" {
		t.Errorf("as is: %+v", p)
	}
}

// A page that advertises a feed, like a Ghost or WordPress blog's home page, shows the feed.
func TestPageThatAdvertisesAFeedShowsTheFeed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/blog/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Blog</title><link rel="alternate" type="application/rss+xml" title="Blog" href="/rss/"></head><body><a href="/x">x</a></body></html>`))
	})
	mux.HandleFunc("/rss/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>The Blog</title><item><title>First post</title><link>https://x.test/1</link></item></channel></rss>`))
	})
	mux.HandleFunc("/plain/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Plain</title></head></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))

	p := b.load(context.Background(), srv.URL+"/blog/")
	if p.Kind != page.KindMarkdown || p.Title != "The Blog" || !strings.Contains(p.HTML, "First post") {
		t.Fatalf("blog page = kind %v title %q error %q", p.Kind, p.Title, p.Error)
	}
	if p.MarkdownOf != srv.URL+"/blog/" {
		t.Errorf("MarkdownOf = %q, so there is no way back to the original page", p.MarkdownOf)
	}
	if p := b.load(context.Background(), srv.URL+"/plain/"); p.Kind != page.KindFallback {
		t.Errorf("a page with no feed should stay a fallback page, got kind %v", p.Kind)
	}
	b.SetMarkdownVersions(false)
	if p := b.load(context.Background(), srv.URL+"/blog/"); p.Kind != page.KindFallback {
		t.Errorf("with the markdown lookup off the feed is not looked for either, got kind %v", p.Kind)
	}
}

func TestPageThatAdvertisesMarkdownShowsIt(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<head><title>Page</title><link rel="alternate" type="text/markdown" href="/elsewhere/source.txt"></head>`))
	})
	mux.HandleFunc("/elsewhere/source.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# From the advertised link\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	p := New(history.Open(""), fetch.NewFetcher(nil, nil)).load(context.Background(), srv.URL+"/page")
	if p.Kind != page.KindMarkdown || p.Title != "From the advertised link" {
		t.Errorf("page = kind %v title %q error %q", p.Kind, p.Title, p.Error)
	}
}
