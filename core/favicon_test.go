package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func svgOf(color string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1" fill="` + color + `"/></svg>`
}

// favicon asks until the icon has been looked for: it is got in the background.
func favicon(t *testing.T, a *Core, pageURL, hint string) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if d := a.Favicon(pageURL, hint); d != "" {
			return d
		}
		time.Sleep(20 * time.Millisecond)
	}
	return ""
}

func iconServer(files map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := files[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "image/svg+xml")
			io.WriteString(w, body)
			return
		}
		http.NotFound(w, r)
	}))
}

func TestFaviconFromTheRootOfTheDomain(t *testing.T) {
	srv := iconServer(map[string]string{"/favicon.ico": svgOf("red")})
	defer srv.Close()
	a := newTestCore(t, t.TempDir())
	a.iconsOff = false
	if d := favicon(t, a, srv.URL+"/notes/a.md", ""); !strings.HasPrefix(d, "data:image/svg+xml;base64,") {
		t.Errorf("icon = %.40q", d)
	}
	if d := a.Favicon("file:///x/a.md", ""); d != "" {
		t.Errorf("a file has an icon: %.30q", d)
	}
}

func TestFaviconNamedInTheFrontMatterComesFirst(t *testing.T) {
	srv := iconServer(map[string]string{"/favicon.ico": svgOf("red"), "/pics/mine.svg": svgOf("blue")})
	defer srv.Close()
	a := newTestCore(t, t.TempDir())
	a.iconsOff = false
	root := favicon(t, a, srv.URL+"/a.md", "")
	// The site's icon is shown while the page's own is fetched; then the page's own takes over.
	mine := ""
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if mine = a.Favicon(srv.URL+"/a.md", srv.URL+"/pics/mine.svg"); mine != "" && mine != root {
			break
		}
	}
	if root == "" || mine == "" || root == mine {
		t.Errorf("the page's own icon should differ from the site's: root %.30q mine %.30q", root, mine)
	}
	inline := "data:image/svg+xml;base64,PHN2Zy8+"
	if d := a.Favicon(srv.URL+"/a.md", inline); d != inline {
		t.Errorf("an icon in the front matter itself = %.30q", d)
	}
}

func TestFaviconInTheFirstFolderOfASubsite(t *testing.T) {
	srv := iconServer(map[string]string{"/scripting/favicon.ico": svgOf("green")}) // none at the root
	defer srv.Close()
	a := newTestCore(t, t.TempDir())
	a.iconsOff = false
	if d := favicon(t, a, srv.URL+"/scripting/2026/11/20.md", ""); !strings.HasPrefix(d, "data:image/svg+xml") {
		t.Errorf("icon = %.40q", d)
	}
	// A page outside that folder does not take its icon.
	if d := a.Favicon(srv.URL+"/other.md", ""); d != "" {
		t.Errorf("another part of the site got %.30q", d)
	}
}
