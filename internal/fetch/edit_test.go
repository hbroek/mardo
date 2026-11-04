package fetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestEditable(t *testing.T) {
	res := func(url string, h map[string]string) *Result {
		hd := http.Header{}
		for k, v := range h {
			hd.Add(k, v)
		}
		return &Result{URL: url, Status: 200, Header: hd}
	}
	cases := []struct {
		name string
		res  *Result
		want string
	}{
		{"put allowed", res("https://x.test/a.md", map[string]string{"Allow": "GET, HEAD, put, DELETE"}), "https://x.test/a.md"},
		{"read only", res("https://x.test/a.md", map[string]string{"Allow": "GET, HEAD"}), ""},
		{"no Allow", res("https://x.test/a.md", nil), ""},
		{"edit link", res("https://x.test/view", map[string]string{"Allow": "GET, PUT", "Link": `<https://x.test/docs/a.md>; rel="edit", </next>; rel=next`}), "https://x.test/docs/a.md"},
		{"relative edit link", res("https://x.test/view", map[string]string{"Allow": "PUT", "Link": `</docs/a.md>; rel="alternate edit"`}), "https://x.test/docs/a.md"},
		{"edit link on another server is ignored", res("https://x.test/a.md", map[string]string{"Allow": "PUT", "Link": `<https://evil.test/a.md>; rel="edit"`}), "https://x.test/a.md"},
		{"error page", &Result{URL: "https://x.test/", Status: 404, Header: http.Header{"Allow": {"PUT"}}}, ""},
		{"file", res("file:///a.md", map[string]string{"Allow": "PUT"}), ""},
	}
	for _, c := range cases {
		if got := Editable(c.res); got != c.want {
			t.Errorf("%s: Editable = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPut(t *testing.T) {
	var gotBody, gotMatch, gotType, method string
	status := http.StatusNoContent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, gotMatch, gotType = r.Method, r.Header.Get("If-Match"), r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.URL.Path == "/moved" {
			http.Redirect(w, r, "/elsewhere", http.StatusFound)
			return
		}
		w.WriteHeader(status)
	}))
	defer srv.Close()
	f := NewFetcher(nil, nil)
	ctx := context.Background()

	if err := f.Put(ctx, srv.URL+"/a.md", []byte("# New"), `"v1"`); err != nil {
		t.Fatal(err)
	}
	if method != "PUT" || gotBody != "# New" || gotMatch != `"v1"` || gotType != "text/markdown; charset=utf-8" {
		t.Errorf("sent %s %q If-Match %q type %q", method, gotBody, gotMatch, gotType)
	}
	f.Put(ctx, srv.URL+"/a.md", []byte("x"), `W/"weak"`)
	if gotMatch != "" {
		t.Errorf("weak ETag sent as If-Match: %q", gotMatch)
	}
	status = http.StatusPreconditionFailed
	if err := f.Put(ctx, srv.URL+"/a.md", []byte("x"), `"v1"`); err != ErrChanged {
		t.Errorf("412 = %v", err)
	}
	status = http.StatusForbidden
	if err := f.Put(ctx, srv.URL+"/a.md", []byte("x"), ""); err == nil {
		t.Error("403 was reported as saved")
	}
	if err := f.Put(ctx, srv.URL+"/moved", []byte("x"), ""); err == nil {
		t.Error("a redirect, which turns the PUT into a GET, was reported as saved")
	}
	if err := f.Put(ctx, "/tmp/a.md", []byte("x"), ""); err == nil {
		t.Error("a local file was PUT")
	}
}

func TestPutAcceptedIsNotSaved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `{"message":"queued for review"}`)
	}))
	defer srv.Close()
	err := NewFetcher(nil, nil).Put(context.Background(), srv.URL+"/a.md", []byte("x"), "")
	var pending *PendingError
	if !errors.Is(err, ErrPending) || !errors.As(err, &pending) || pending.Note != "queued for review" || !strings.Contains(err.Error(), "queued for review") {
		t.Errorf("202 = %v", err)
	}
}

func TestLocalFileEditAndConflict(t *testing.T) {
	path := t.TempDir() + "/note.md"
	if err := os.WriteFile(path, []byte("# one\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	res, err := NewFetcher(nil, nil).Get(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	edit := Editable(res)
	tag := res.Header.Get("ETag")
	if edit != res.URL || tag == "" {
		t.Fatalf("a writable file should be editable: edit=%q tag=%q", edit, tag)
	}
	if err := PutFile(edit, []byte("# two\n"), tag); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "# two\n" {
		t.Fatalf("file holds %q", got)
	}
	if info, _ := os.Stat(path); runtime.GOOS != "windows" && info.Mode().Perm() != 0o640 { // Windows has no permission bits to keep
		t.Fatalf("mode changed to %v", info.Mode().Perm())
	}
	if err := PutFile(edit, []byte("# three\n"), tag); !errors.Is(err, ErrChanged) {
		t.Fatalf("saving over a newer file should fail with ErrChanged, got %v", err)
	}
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if os.Getuid() != 0 && runtime.GOOS != "windows" {
		res, _ = NewFetcher(nil, nil).Get(context.Background(), path)
		if Editable(res) != "" {
			t.Fatal("a read-only file should not be editable")
		}
	}
}
