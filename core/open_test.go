package core

import (
	"path/filepath"
	"testing"
)

func TestOpenRequests(t *testing.T) {
	a := newCore(t, Config{})
	if got := a.TakeOpenRequests(); got == nil || len(got) != 0 {
		t.Fatalf("nothing queued: %#v", got)
	}
	QueueOpen(a, "/a/one.md")
	QueueOpen(a, "/a/two.md")
	if got := a.TakeOpenRequests(); len(got) != 2 || got[0] != "/a/one.md" {
		t.Errorf("queued = %v", got)
	}
	if got := a.TakeOpenRequests(); len(got) != 0 {
		t.Errorf("taken twice: %v", got)
	}
}

func TestOpenablePath(t *testing.T) {
	for arg, want := range map[string]bool{"notes.md": true, "/x/y.markdown": true, "/usr/bin/mardo": false, "--flag": false, "a.txt": true} {
		if _, ok := OpenablePath("/w", arg); ok != want {
			t.Errorf("OpenablePath(%q) = %v", arg, ok)
		}
	}
	if p, _ := OpenablePath("/w", "n.md"); p != filepath.Join("/w", "n.md") { // with backslashes on Windows
		t.Errorf("relative = %q", p)
	}
}
