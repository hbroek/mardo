package browser

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mardo/internal/fetch"
	"mardo/internal/history"
	"mardo/internal/page"
)

// An HTML page whose <link rel="alternate" type="text/markdown"> holds the markdown in a data: address.
func TestMarkdownAlternateInADataAddress(t *testing.T) {
	md := "# From the data link\n\nSee [the next post](next.md) and [home](/).\n"
	mux := http.NewServeMux()
	mux.HandleFunc("/posts/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><html><head><title>A</title><link rel="alternate" type="text/markdown" href="data:text/markdown;charset=utf-8;base64,` +
			base64.StdEncoding.EncodeToString([]byte(md)) + `"></head><body><h1>A</h1></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	id := b.NewTab()
	p := b.Navigate(context.Background(), id, srv.URL+"/posts/a")
	if p.Kind != page.KindMarkdown || !strings.Contains(p.HTML, "From the data link") || p.MarkdownOf != srv.URL+"/posts/a" {
		t.Fatalf("the markdown in the link is the page: %+v", p)
	}
	// Its links are the HTML page's: relative to /posts/a, not to a data: address.
	for _, want := range []string{`href="` + srv.URL + `/posts/next.md"`, `href="` + srv.URL + `/"`} {
		if !strings.Contains(p.HTML, want) {
			t.Errorf("missing %s in %s", want, p.HTML)
		}
	}
}
