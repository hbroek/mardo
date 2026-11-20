package core

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func writeAction(t *testing.T, a *Core, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(a.actionsFolder(), name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

const weftPage = "---\ntitle: LinkaGoGo\ndescription: \"LinkaGoGo — Manage your bookmarks\"\nrightMarginImage: https://www.linkagogo.com/img/linkagogo.svg\n---\n\n# LinkaGoGo\n"

func TestActionsAreInstalledOnceAndRun(t *testing.T) {
	a := newTestCore(t, t.TempDir())
	got := map[string]ActionInfo{}
	for _, x := range a.Actions() {
		got[x.ID] = x
	}
	if got["copy-markdown-link"].Mode != "copy" || got["quote-selection"].Name != "Copy selection as a quote" || got["search-google"].Mode != "browser" || got["ask-claude"].Mode != "browser" || got["ask-chatgpt"].Mode != "browser" {
		t.Fatalf("the examples were not installed: %+v", got)
	}
	if err := os.Remove(filepath.Join(a.actionsFolder(), "quote-selection.md")); err != nil {
		t.Fatal(err)
	}
	installActions(a.actionsFolder()) // the folder is the reader's now: a deleted example stays deleted
	if len(a.Actions()) != 4 {
		t.Errorf("a deleted example came back: %+v", a.Actions())
	}
	r, err := a.RunAction("copy-markdown-link", ActionInput{URL: "https://x.test/a", Title: "A [post]"})
	if err != nil || r.Mode != "copy" || r.Text != "[A [post]](https://x.test/a)" {
		t.Errorf("copy = %+v, %v", r, err)
	}
}

func TestActionOpensTheBrowserWithEncodedValues(t *testing.T) {
	a := newTestCore(t, t.TempDir())
	writeAction(t, a, "weft-it.md", "---\nname: Weft it\nicon: 🧵\nurl: https://weft.example/compose?page={{url}}&ptitle={{title}}&pdesc={{description}}&pimg={{image}}&keywords={{keywords}}&wm=0\n---\n")
	var info ActionInfo
	for _, x := range a.Actions() {
		if x.ID == "weft-it" {
			info = x
		}
	}
	if info.Name != "Weft it" || info.Mode != "browser" || info.Icon != "🧵" || info.Host != "weft.example" {
		t.Fatalf("info = %+v", info)
	}
	r, err := a.RunAction("weft-it", ActionInput{URL: "https://www.linkagogo.com/app/#all", Title: "LinkaGoGo", Source: weftPage})
	want := "https://weft.example/compose?page=https%3A%2F%2Fwww.linkagogo.com%2Fapp%2F%23all&ptitle=LinkaGoGo&pdesc=LinkaGoGo%20%E2%80%94%20Manage%20your%20bookmarks&pimg=https%3A%2F%2Fwww.linkagogo.com%2Fimg%2Flinkagogo.svg&keywords=&wm=0"
	if err != nil || r.Mode != "open" || r.URL != want {
		t.Errorf("result = %+v, %v\nwant %s", r, err, want)
	}
}

func TestActionCallsAnAPIWithAJSONBody(t *testing.T) {
	var method, ctype, body, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		method, ctype, body, path = r.Method, r.Header.Get("Content-Type"), string(b), r.URL.RequestURI()
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"ok":true}`)
	}))
	defer srv.Close()
	a := newTestCore(t, t.TempDir())
	writeAction(t, a, "post.md", "---\nname: Post it\nmethod: POST\nurl: "+srv.URL+"/api/posts?from=mardo\n---\n{\"link\": \"{{url}}\", \"title\": \"{{title}}\", \"note\": \"{{selection}}\"}\n")
	r, err := a.RunAction("post", ActionInput{URL: "https://x.test/a", Title: `He said "hi"`, Selection: "line one\nline two"})
	if err != nil || r.Mode != "done" || !strings.Contains(r.Message, "Post it: sent") {
		t.Fatalf("result = %+v, %v", r, err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("the body is not JSON: %q (%v)", body, err)
	}
	if method != "POST" || ctype != "application/json" || path != "/api/posts?from=mardo" ||
		got["link"] != "https://x.test/a" || got["title"] != `He said "hi"` || got["note"] != "line one\nline two" {
		t.Errorf("%s %s %s body %q", method, path, ctype, body)
	}
}

func TestActionReportsAServerRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "not yours", http.StatusForbidden) }))
	defer srv.Close()
	a := newTestCore(t, t.TempDir())
	writeAction(t, a, "put.md", "---\nname: Save\nmethod: PUT\nurl: "+srv.URL+"/x\n---\ntext {{title}}\n")
	_, err := a.RunAction("put", ActionInput{URL: "https://x.test/", Title: "T"})
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "credentials") {
		t.Errorf("err = %v", err)
	}
}

func TestActionFilesThatCannotBeUsedAreLeftOut(t *testing.T) {
	a := newTestCore(t, t.TempDir())
	writeAction(t, a, "nofront.md", "no front matter\n")
	writeAction(t, a, "badmethod.md", "---\nmethod: TRACE\nurl: https://x.test/\n---\n")
	writeAction(t, a, "browserpost.md", "---\nmethod: POST\nmode: browser\nurl: https://x.test/\n---\n")
	writeAction(t, a, "js.md", "---\nurl: javascript:alert(1)\n---\n")
	for _, x := range a.Actions() {
		if !strings.Contains("copy-markdown-link quote-selection search-google ask-claude ask-chatgpt", x.ID) {
			t.Errorf("%s was listed: %+v", x.ID, x)
		}
	}
	if _, err := a.RunAction("nofront", ActionInput{}); err == nil {
		t.Error("an unusable action ran")
	}
}

func TestActionVarsFromAPage(t *testing.T) {
	v := actionVars(ActionInput{URL: "https://x.test/dir/a.md", Source: "---\nauthor: Dave\nsource:\n  title: SN\n---\n\n![Lab](lab.png)\n\nThe **first** [paragraph](https://y.test) of text.\n"}, time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC))
	if v["description"] != "The first paragraph of text." || v["image"] != "https://x.test/dir/lab.png" ||
		v["page.author"] != "Dave" || v["page.source.title"] != "SN" || v["host"] != "x.test" || v["title"] != "x.test" {
		t.Errorf("vars = %v", v)
	}
}

func TestActionPlaceholdersBodyFullbodyAndAnyKey(t *testing.T) {
	a := newTestCore(t, t.TempDir())
	writeAction(t, a, "all.md", "---\nname: All\nmode: copy\n---\n[{{body}}] [{{fullbody}}] {{author}}/{{page.author}}/{{source.title}}/{{Missing}}/{{page.nothing}}|{{description}}|\n")
	src := "---\ntitle: T\nauthor: Dave\nsource:\n  title: SN\n---\n\nHello there.\n"
	r, err := a.RunAction("all", ActionInput{URL: "https://x.test/a", Title: "T", Source: src})
	want := "[\nHello there.\n] [" + src + "] Dave/Dave/SN//|Hello there.|"
	if err != nil || r.Text != want {
		t.Errorf("text = %q, %v\nwant %q", r.Text, err, want)
	}
	// A page that is not markdown has no body and no keys: everything is empty, and nothing is left as written.
	r, _ = a.RunAction("all", ActionInput{URL: "https://x.test/a", Title: "T"})
	if r.Text != "[] [] ////||" {
		t.Errorf("without markdown: %q", r.Text)
	}
}

func TestSearchActionNeedsASelectionAndEncodesIt(t *testing.T) {
	a := newTestCore(t, t.TempDir())
	if _, err := a.RunAction("search-google", ActionInput{URL: "https://x.test/", Title: "T"}); err == nil || !strings.Contains(err.Error(), "select some text") {
		t.Errorf("without a selection: %v", err)
	}
	r, err := a.RunAction("search-google", ActionInput{URL: "https://x.test/", Selection: "rust & go: why?"})
	if err != nil || r.URL != "https://www.google.com/search?q=rust%20%26%20go%3A%20why%3F" {
		t.Errorf("google = %+v, %v", r, err)
	}
	r, err = a.RunAction("ask-claude", ActionInput{URL: "https://x.test/a", Selection: "hello"})
	if err != nil || r.URL != "https://claude.ai/new?q=Explain%20this%20passage%20from%20https%3A%2F%2Fx.test%2Fa:%20hello" {
		t.Errorf("claude = %+v, %v", r, err)
	}
}

func TestActionsAreOrderedByTheOrderKey(t *testing.T) {
	a := newTestCore(t, t.TempDir())
	for _, x := range a.Actions() { // the examples, which have orders
		_ = x
	}
	writeAction(t, a, "zz-first.md", "---\nname: Zed\norder: 1\nmode: copy\n---\nx\n")
	writeAction(t, a, "aa-last.md", "---\nname: Aardvark\nmode: copy\n---\nx\n")
	writeAction(t, a, "mid.md", "---\nname: Middle\norder: 15\nmode: copy\n---\nx\n")
	var ids []string
	for _, x := range a.Actions() {
		ids = append(ids, x.ID)
	}
	got := strings.Join(ids, " ")
	if !strings.HasPrefix(got, "zz-first search-google mid ask-claude ask-chatgpt copy-markdown-link quote-selection ") || !strings.HasSuffix(got, "aa-last") {
		t.Errorf("order = %s", got)
	}
}

func TestIconTypeComesFromTheContent(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`)
	if got := imageDataURL("https://x.test/favicon.ico", "image/svg+xml", svg); !strings.HasPrefix(got, "data:image/svg+xml;base64,") {
		t.Errorf("an svg named .ico = %.40q", got)
	}
	ico := []byte{0, 0, 1, 0, 1, 0, 16, 16, 0, 0, 1, 0, 32, 0}
	if got := imageDataURL("https://x.test/favicon.ico", "", ico); !strings.HasPrefix(got, "data:image/") {
		t.Errorf("an ico = %.40q", got)
	}
	if got := imageDataURL("https://x.test/favicon.ico", "text/html", []byte("<html>not found</html>")); got != "" {
		t.Errorf("a page was taken for a picture: %.40q", got)
	}
}

func TestIconsAreKeptAcrossRestarts(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/svg+xml")
		io.WriteString(w, svg)
	}))
	defer srv.Close()
	dir := t.TempDir()
	a := newTestCore(t, dir)
	a.iconsOff = false
	clearActions(t, a)
	writeAction(t, a, "fav.md", "---\nname: Fav\nicon: favicon\nmode: copy\nurl: "+srv.URL+"/x\n---\nx\n")
	icon := func(c *Core) string {
		for _, x := range c.Actions() {
			if x.ID == "fav" {
				return x.Icon
			}
		}
		return ""
	}
	// The picture is got in the background; the first answer has none.
	deadline := time.Now().Add(3 * time.Second)
	for icon(a) == "" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.HasPrefix(icon(a), "data:image/svg+xml;base64,") {
		t.Fatalf("no icon: %q", icon(a))
	}
	// A new core on the same folder has it at once, without asking the server again.
	before := hits.Load()
	a.bmMu.Lock() // the first one is closed, as it is when the app is
	a.bookmarks.Close()
	a.bmMu.Unlock()
	b := newTestCore(t, dir)
	b.iconsOff = false
	if !strings.HasPrefix(icon(b), "data:image/svg+xml;base64,") || hits.Load() != before {
		t.Errorf("after a restart: icon %.30q, %d new requests", icon(b), hits.Load()-before)
	}
}

func TestPageIconsBestFirst(t *testing.T) {
	html := `<html><head>
<link rel="apple-touch-icon" href="/touch.png" sizes="180x180">
<link rel="icon" href="/big.png" type="image/png" sizes="192x192">
<link rel="shortcut icon" href="/old.ico">
<link rel="icon" href="/cdn/favicon.svg" type="image/svg+xml">
<link rel="icon" href="/p48.png" type="image/png" sizes="48x48">
<link rel="icon" href="/p32.png" type="image/png" sizes="32x32">
<link rel="stylesheet" href="/x.css"></head><body></body></html>`
	base, _ := url.Parse("https://chatgpt.com/")
	got := strings.Join(pageIcons([]byte(html), base), " ")
	want := "https://chatgpt.com/cdn/favicon.svg https://chatgpt.com/p32.png https://chatgpt.com/p48.png https://chatgpt.com/old.ico https://chatgpt.com/big.png https://chatgpt.com/touch.png"
	if got != want {
		t.Errorf("icons = %s\nwant    %s", got, want)
	}
}

func TestFaviconIsFoundInThePageWhenThereIsNoFaviconIco(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, `<html><head><link rel="icon" href="/cdn/f.svg" type="image/svg+xml"></head></html>`)
		case "/cdn/f.svg":
			w.Header().Set("Content-Type", "image/svg+xml")
			io.WriteString(w, svg)
		default:
			http.NotFound(w, r) // no /favicon.ico
		}
	}))
	defer srv.Close()
	a := newTestCore(t, t.TempDir())
	a.iconsOff = false
	clearActions(t, a)
	writeAction(t, a, "fav.md", "---\nname: Fav\nicon: favicon\nmode: copy\nurl: "+srv.URL+"/x\n---\nx\n")
	deadline := time.Now().Add(3 * time.Second)
	icon := ""
	for icon == "" && time.Now().Before(deadline) {
		for _, x := range a.Actions() {
			if x.ID == "fav" {
				icon = x.Icon
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.HasPrefix(icon, "data:image/svg+xml;base64,") {
		t.Errorf("icon = %.40q", icon)
	}
}

// clearActions empties the actions folder, so that the examples do not fetch their icons from the web.
func clearActions(t *testing.T, a *Core) {
	t.Helper()
	if err := os.RemoveAll(a.actionsFolder()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.actionsFolder(), 0o755); err != nil {
		t.Fatal(err)
	}
}
