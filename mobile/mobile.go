// Package mobile is the Go core packed for an iPhone app (gomobile bind makes a framework of it).
//
// The web interface runs in a WKWebView and calls the core the way it does on the desktop, by method
// name with a JSON list of arguments; Swift forwards each call to Engine.Call and sends the answer
// back. Everything the core wants from the device (file pickers, the keychain, telling the page that
// something happened) goes through the small interfaces below, which the Swift side implements.
// Only simple types cross the boundary, which is why the data travels as JSON strings.
package mobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"reflect"

	"mardo/core"
	"mardo/internal/auth"
)

// Host is the device's side of the core's Host, in types gomobile can carry. Filters come as JSON:
// [{"Name": "...", "Pattern": "*.md;*.txt"}].
type Host interface {
	// PickOpen asks for a file to read and returns where it is, or "" when canceled.
	PickOpen(title, filtersJSON string) (string, error)
	// PickSave asks where to write a file and returns the path, or "" when canceled.
	PickSave(title, dir, name, filtersJSON string) (string, error)
	// Emit tells the page that something happened; data is JSON.
	Emit(event, dataJSON string)
	// SetChecked ticks a menu item; a phone has no menu bar and can ignore it.
	SetChecked(id string, on bool)
}

// Secrets is the keychain. Get answers "" when nothing is stored under the name.
type Secrets interface {
	Get(name string) string
	Set(name, value string) error
	Delete(name string)
}

// Engine is the core with its way of being called.
type Engine struct {
	core    *core.Core
	methods map[string]reflect.Value
}

// New starts the core. dir is where its files live (the app's own folder, or the one it shares with
// an extension).
func New(dir string, secrets Secrets, host Host) *Engine {
	e := &Engine{methods: map[string]reflect.Value{}}
	e.core = core.New(core.Config{Dir: dir, Secrets: secretStore{secrets}, Host: hostAdapter{host}})
	v := reflect.ValueOf(e.core)
	for i := 0; i < v.NumMethod(); i++ {
		e.methods[v.Type().Method(i).Name] = v.Method(i)
	}
	return e
}

// Call runs one method of the core. args is a JSON list ("[]" or "" for none). The answer is
// {"ok": <result>} or {"error": "<message>"}; a method with no result answers {"ok": null}.
func (e *Engine) Call(method, argsJSON string) string {
	out, err := e.call(method, argsJSON)
	if err != nil {
		b, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(b)
	}
	b, err := json.Marshal(map[string]any{"ok": out})
	if err != nil {
		b, _ = json.Marshal(map[string]string{"error": err.Error()})
	}
	return string(b)
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func (e *Engine) call(method, argsJSON string) (result any, err error) {
	defer func() { // a panic in one call must not take the app down
		if r := recover(); r != nil {
			result, err = nil, fmt.Errorf("%s failed: %v", method, r)
		}
	}()
	fn, ok := e.methods[method]
	if !ok {
		return nil, fmt.Errorf("unknown method %q", method)
	}
	var raw []json.RawMessage
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
			return nil, fmt.Errorf("%s: the arguments must be a JSON list", method)
		}
	}
	t := fn.Type()
	if len(raw) != t.NumIn() {
		return nil, fmt.Errorf("%s takes %d arguments, got %d", method, t.NumIn(), len(raw))
	}
	in := make([]reflect.Value, t.NumIn())
	for i := range in {
		p := reflect.New(t.In(i))
		if err := json.Unmarshal(raw[i], p.Interface()); err != nil {
			return nil, fmt.Errorf("%s: argument %d: %v", method, i+1, err)
		}
		in[i] = p.Elem()
	}
	out := fn.Call(in)
	if n := len(out); n > 0 && out[n-1].Type() == errorType {
		if err, _ := out[n-1].Interface().(error); err != nil {
			return nil, err
		}
		out = out[:n-1]
	}
	switch len(out) {
	case 0:
		return nil, nil
	case 1:
		return out[0].Interface(), nil
	}
	list := make([]any, len(out))
	for i, o := range out {
		list[i] = o.Interface()
	}
	return list, nil
}

// Asset is an answer to a request the web view made for an image (see Engine.Asset).
type Asset struct {
	Status int
	Type   string
	Body   []byte
}

// Asset answers the web view's request for path (such as "/imgproxy/..." for a web image that needs
// this app's credentials): Swift's URL scheme handler forwards those requests here.
func (e *Engine) Asset(path string) (*Asset, error) {
	if path == "" || path[0] != '/' {
		return nil, errors.New("the path must start with /")
	}
	rec := httptest.NewRecorder()
	core.AssetHandler(e.core).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), "GET", path, nil))
	res := rec.Result()
	defer res.Body.Close()
	return &Asset{Status: res.StatusCode, Type: res.Header.Get("Content-Type"), Body: rec.Body.Bytes()}, nil
}

type hostAdapter struct{ h Host }

func filtersJSON(fs []core.Filter) string {
	b, _ := json.Marshal(fs)
	return string(b)
}

func (a hostAdapter) PickOpen(title string, fs []core.Filter) (string, error) {
	return a.h.PickOpen(title, filtersJSON(fs))
}

func (a hostAdapter) PickSave(title, dir, name string, fs []core.Filter) (string, error) {
	return a.h.PickSave(title, dir, name, filtersJSON(fs))
}

func (a hostAdapter) Emit(event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	a.h.Emit(event, string(b))
}

func (a hostAdapter) SetChecked(id string, on bool) { a.h.SetChecked(id, on) }

type secretStore struct{ s Secrets }

func (s secretStore) Get(name string) (string, error) {
	if v := s.s.Get(name); v != "" {
		return v, nil
	}
	return "", auth.ErrNotFound
}

func (s secretStore) Set(name, value string) error { return s.s.Set(name, value) }

func (s secretStore) Delete(name string) error {
	s.s.Delete(name)
	return nil
}
