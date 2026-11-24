package core

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mardo/internal/fetch"
)

func TestWriteNewDocumentAddsExtension(t *testing.T) {
	dir := t.TempDir()
	for name, want := range map[string]string{"notes": "notes.md", "a.md": "a.md", "b.txt": "b.txt", "c.markdown": "c.markdown", "d.html": "d.html.md"} {
		got, err := writeNewDocument(filepath.Join(dir, name), "# Hi\n")
		if err != nil || filepath.Base(got) != want {
			t.Fatalf("%s: got %q, %v; want %s", name, got, err, want)
		}
		if b, _ := os.ReadFile(got); string(b) != "# Hi\n" {
			t.Fatalf("%s holds %q", name, b)
		}
	}
}

func TestSaveSourceTellsWhenTheServerOnlyQueuedTheChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"message":"waiting for review"}`)
			return
		}
		w.Header().Set("Content-Type", "text/markdown")
		w.Header().Set("Allow", "GET, PUT")
		io.WriteString(w, "# Old\n")
	}))
	defer srv.Close()
	a := newCore(t, Config{})
	SetContext(a, context.Background())
	tab := a.NewTab()
	a.Navigate(tab, srv.URL+"/a.md")
	p, err := a.SaveSource(tab, srv.URL+"/a.md", srv.URL+"/a.md", "", "# New\n")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Submitted || p.SubmittedNote != "waiting for review" || p.Kind != "markdown" {
		t.Errorf("page after a queued save: submitted=%v note=%q kind=%q", p.Submitted, p.SubmittedNote, p.Kind)
	}
}

func TestEditingADataPageMakesANewDataAddress(t *testing.T) {
	a := newTestCore(t, t.TempDir())
	p := a.Navigate(0, fetch.DataURL("# Old\n"))
	if p.EditURL == "" {
		t.Fatalf("a data page cannot be edited: %+v", p)
	}
	fresh, err := a.SaveSource(0, p.URL, p.EditURL, p.ETag, "# New\n")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.URL != fetch.DataURL("# New\n") || !strings.Contains(fresh.HTML, "New") {
		t.Errorf("page after the edit: url %q html %q", fresh.URL, fresh.HTML)
	}
}

// A server that allows POST on a page is told so in the page; the new page is added with POST and shown, and a page
// that allows DELETE can be removed.
func TestAddingAndDeletingPagesOnAServer(t *testing.T) {
	var posted, deleted, ifMatch, contentType string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/":
			b, _ := io.ReadAll(r.Body)
			posted, contentType = string(b), r.Header.Get("Content-Type")
			if strings.Contains(posted, "bad") {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"message":"line 2: no title"}`)
				return
			}
			w.Header().Set("Location", "/posts/new-one.k3x9")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodDelete:
			deleted, ifMatch = r.URL.Path, r.Header.Get("If-Match")
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/markdown")
			w.Header().Set("Allow", "GET, HEAD, POST")
			io.WriteString(w, "# Home\n")
		case r.URL.Path == "/posts/new-one.k3x9" && deleted != "":
			http.Error(w, "gone", http.StatusGone)
		default:
			w.Header().Set("Content-Type", "text/markdown")
			w.Header().Set("Allow", "GET, HEAD, PUT, DELETE")
			w.Header().Set("ETag", `"v1"`)
			io.WriteString(w, "# New one\n")
		}
	}))
	defer srv.Close()
	a := newCore(t, Config{})
	SetContext(a, context.Background())
	tab := a.NewTab()
	home := a.Navigate(tab, srv.URL+"/")
	if !home.CanAdd || home.CanDelete {
		t.Fatalf("the home page allows POST only: %+v", home)
	}
	if _, err := a.AddPage(tab, srv.URL+"/", "---\ntitle: bad\n---\n"); err == nil || !strings.Contains(err.Error(), "line 2: no title") {
		t.Errorf("the server's message is shown: %v", err)
	}
	p, err := a.AddPage(tab, srv.URL+"/", "---\ntitle: New one\n---\nHi\n")
	if err != nil || !strings.HasSuffix(p.URL, "/posts/new-one.k3x9") || contentType != "text/markdown; charset=utf-8" {
		t.Fatalf("created: %+v, %v, %q", p, err, contentType)
	}
	if !p.CanDelete || p.CanAdd || p.EditURL == "" {
		t.Errorf("the new post can be changed and deleted: %+v", p)
	}
	gone, err := a.DeletePage(tab, p.URL, p.ETag)
	if err != nil || deleted != "/posts/new-one.k3x9" || ifMatch != `"v1"` || gone.Kind != "error" {
		t.Errorf("deleted: %+v, %v, %q, %q", gone, err, deleted, ifMatch)
	}
}
