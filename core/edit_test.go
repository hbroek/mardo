package core

import (
	"os"
	"path/filepath"
	"testing"
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
