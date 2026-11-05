package mobile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeHost struct {
	open, save string
	events     []string
	filters    string
}

func (h *fakeHost) PickOpen(title, filters string) (string, error) {
	h.filters = filters
	return h.open, nil
}
func (h *fakeHost) PickSave(title, dir, name, filters string) (string, error) {
	return h.save, nil
}
func (h *fakeHost) Emit(event, data string) { h.events = append(h.events, event+" "+data) }
func (h *fakeHost) SetChecked(string, bool) {}

func (h *fakeHost) AIStatus() string { return "" }

func (h *fakeHost) AIChat(string) (string, error) { return "from the device", nil }

type fakeSecrets map[string]string

func (s fakeSecrets) Get(name string) string       { return s[name] }
func (s fakeSecrets) Set(name, value string) error { s[name] = value; return nil }
func (s fakeSecrets) Delete(name string)           { delete(s, name) }

type answer struct {
	OK    json.RawMessage `json:"ok"`
	Error string          `json:"error"`
}

func call(t *testing.T, e *Engine, method, args string) answer {
	t.Helper()
	var a answer
	if err := json.Unmarshal([]byte(e.Call(method, args)), &a); err != nil {
		t.Fatalf("%s: answer is not JSON: %v", method, err)
	}
	return a
}

func TestCallNavigatesAndReturnsThePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown")
		fmt.Fprint(w, "# Hello phone\n")
	}))
	defer srv.Close()
	host := &fakeHost{}
	e := New("", fakeSecrets{}, host)

	var tab int
	if a := call(t, e, "NewTab", "[]"); json.Unmarshal(a.OK, &tab) != nil || a.Error != "" {
		t.Fatalf("NewTab = %+v", a)
	}
	a := call(t, e, "Navigate", fmt.Sprintf(`[%d, %q]`, tab, srv.URL+"/"))
	var p struct{ Title, Kind, HTML string }
	if err := json.Unmarshal(a.OK, &p); err != nil || a.Error != "" || p.Title != "Hello phone" || !strings.Contains(p.HTML, "<h1") {
		t.Fatalf("Navigate = %s (%v)", a.OK, err)
	}
	if len(host.events) == 0 || !strings.HasPrefix(host.events[0], "net ") { // the request reached the console
		t.Errorf("events = %v", host.events)
	}
}

func TestCallErrors(t *testing.T) {
	e := New("", fakeSecrets{}, &fakeHost{})
	for _, c := range []struct{ method, args, want string }{
		{"NoSuchMethod", "[]", "unknown method"},
		{"Navigate", "[1]", "takes 2 arguments"},
		{"Navigate", `["x", "y"]`, "argument 1"},
		{"Navigate", "not json", "JSON list"},
		{"DeleteBookmark", `["nope"]`, ""}, // some error from the store, whatever its text
	} {
		a := call(t, e, c.method, c.args)
		if a.Error == "" || !strings.Contains(a.Error, c.want) {
			t.Errorf("%s %s: error %q, want it to contain %q", c.method, c.args, a.Error, c.want)
		}
	}
	if a := call(t, e, "ClearHistory", ""); a.Error != "" || string(a.OK) != "null" {
		t.Errorf("a method with no result: %+v", a)
	}
}

func TestSecretsAndPickers(t *testing.T) {
	secrets := fakeSecrets{}
	host := &fakeHost{open: "/tmp/in.md"}
	e := New("", secrets, host)

	before := len(secrets) // the cookie jar already keeps its key there
	a := call(t, e, "SaveCredential", `[{"origin":"https://example.com","kind":"bearer","token":"s3cret"}]`)
	if a.Error != "" || len(secrets) != before+1 {
		t.Fatalf("SaveCredential = %+v, secrets %v", a, secrets)
	}
	if a := call(t, e, "PickFile", "[]"); string(a.OK) != `"/tmp/in.md"` || !strings.Contains(host.filters, "*.md") {
		t.Errorf("PickFile = %+v, filters %s", a, host.filters)
	}
}

func TestAssetServesTheHandler(t *testing.T) {
	e := New("", fakeSecrets{}, &fakeHost{})
	if _, err := e.Asset("relative"); err == nil {
		t.Error("a path without a leading / is refused")
	}
	if a, err := e.Asset("/imgproxy/"); err != nil || a.Status == 0 {
		t.Errorf("Asset = %+v, %v", a, err)
	}
}
