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
