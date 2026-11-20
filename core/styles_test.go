package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestCore starts a core on dir and closes its bookmark database when the test ends: Windows cannot delete the
// temporary folder while the file is open.
func newTestCore(t *testing.T, dir string) *Core {
	t.Helper()
	a := New(Config{Dir: dir})
	t.Cleanup(func() {
		a.bmMu.Lock()
		defer a.bmMu.Unlock()
		if a.bookmarks != nil {
			a.bookmarks.Close()
		}
	})
	return a
}

func TestBuiltInStylesAreCopiedOnceAndListed(t *testing.T) {
	dir := t.TempDir()
	a := newTestCore(t, dir)
	folder := a.StylesFolder()
	var names []string
	for _, s := range a.StyleSheets() {
		names = append(names, s.Name)
	}
	if strings.Join(names, ",") != "Basic,Book,Classic,Mono,Newsprint" {
		t.Fatalf("styles = %v", names)
	}
	// An edited style stays as it is at the next start, a deleted one comes back, and a style of your own is listed.
	os.WriteFile(filepath.Join(folder, "book.css"), []byte("/* mine */"), 0o644)
	os.Remove(filepath.Join(folder, "mono.css"))
	os.WriteFile(filepath.Join(folder, "my-style.css"), []byte("body{}"), 0o644)
	os.WriteFile(filepath.Join(folder, "notes.txt"), []byte("x"), 0o644)
	a = newTestCore(t, dir)
	if css, _ := a.StyleSheet("book"); css != "/* mine */" {
		t.Errorf("an edited style was replaced: %q", css)
	}
	var ids []string
	for _, s := range a.StyleSheets() {
		ids = append(ids, s.ID)
	}
	if strings.Join(ids, ",") != "basic,book,classic,mono,my-style,newsprint" {
		t.Errorf("ids = %v", ids)
	}
	if s := a.StyleSheets()[4]; s.Name != "My style" {
		t.Errorf("name = %q", s.Name)
	}
}

func TestStyleSheetBuildsInImportsAndFontsAndNothingFromTheNetwork(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "stylesheets")
	os.MkdirAll(filepath.Join(folder, "fonts"), 0o755)
	os.WriteFile(filepath.Join(folder, "fonts", "f.css"), []byte(`@font-face { font-family: F; src: url("f.woff2") format("woff2"); }`), 0o644)
	os.WriteFile(filepath.Join(folder, "fonts", "f.woff2"), []byte("FONT"), 0o644)
	os.WriteFile(filepath.Join(folder, "secret.txt"), []byte("no"), 0o644)
	os.WriteFile(filepath.Join(folder, "mine.css"), []byte(`@import url("fonts/f.css");
@import url("https://fonts.example/x.css");
@import "../outside.css";
body { background: url(https://tracker.example/p.png), url("/etc/passwd"), url(secret.txt); font-family: F; }`), 0o644)
	os.WriteFile(filepath.Join(dir, "outside.css"), []byte("body{color:red}"), 0o644)
	a := newTestCore(t, dir)
	css, err := a.StyleSheet("mine")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(css, "font-family: F; src: url(\"data:font/woff2;base64,Rk9OVA==\")") {
		t.Errorf("the font is built in: %s", css)
	}
	for _, not := range []string{"tracker.example", "fonts.example", "/etc/passwd", "secret.txt", "color:red", "@import"} {
		if strings.Contains(css, not) {
			t.Errorf("%q should be gone: %s", not, css)
		}
	}
	if _, err := a.StyleSheet("../x"); err == nil {
		t.Error("a name with a path is not a style")
	}
}
