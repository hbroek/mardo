package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, d := range []string{"Zeta", "alpha", ".hidden"} {
		if err := os.Mkdir(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"b.md", "A.TXT", "c.markdown", "pic.png", "notes.docx", ".dot.md"} {
		if err := os.WriteFile(filepath.Join(home, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := &Core{}
	list, err := c.ListFolder("")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range list.Entries {
		names = append(names, e.Name)
	}
	want := []string{"alpha", "Zeta", "A.TXT", "b.md", "c.markdown"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
	if !list.Entries[0].Folder || list.Entries[2].URL == "" {
		t.Errorf("folder/url flags wrong: %+v", list.Entries)
	}
	if _, err := c.ListFolder(filepath.Dir(home)); err == nil {
		t.Error("listing above home should fail")
	}
	if _, err := c.ListFolder(filepath.Join(home, "alpha", "..", "..")); err == nil {
		t.Error("a .. escape should fail")
	}
	if l, err := c.ListFolder(filepath.Join(home, "alpha")); err != nil || len(l.Entries) != 0 {
		t.Errorf("empty folder: %v %v", l, err)
	}
}

func TestRenamePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.Mkdir(filepath.Join(home, "dir"), 0o755)
	os.WriteFile(filepath.Join(home, "a.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(home, "b.md"), []byte("y"), 0o644)
	c := &Core{}

	got, err := c.RenamePath(filepath.Join(home, "a.md"), "notes")
	if err != nil || got != filepath.Join(home, "notes.md") {
		t.Fatalf("keep extension: %q %v", got, err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Error("renamed file missing")
	}
	if got, err := c.RenamePath(filepath.Join(home, "dir"), "folder.d"); err != nil || got != filepath.Join(home, "folder.d") {
		t.Errorf("folder: %q %v", got, err)
	}
	for _, tc := range []struct{ path, name string }{
		{filepath.Join(home, "notes.md"), "b.md"},     // exists
		{filepath.Join(home, "notes.md"), "../x.md"},  // leaves the folder
		{filepath.Join(home, "notes.md"), "pic.png"},  // would vanish from the list
		{filepath.Join(home, "notes.md"), " "},        // empty
		{home, "other"},                               // the home folder itself
		{filepath.Join(filepath.Dir(home), "x"), "y"}, // outside
	} {
		if _, err := c.RenamePath(tc.path, tc.name); err == nil {
			t.Errorf("RenamePath(%q, %q) should fail", tc.path, tc.name)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(home, "b.md")); string(b) != "y" {
		t.Error("b.md was overwritten")
	}
}
