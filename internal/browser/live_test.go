package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"mardo/internal/fetch"
	"mardo/internal/history"
)

// A site with a urlUpdateSocket: the tab sends "watch <page>", and "update\r<page>" makes Browser.OnUpdate fire.
func TestLiveUpdateSocket(t *testing.T) {
	watched := make(chan string, 4)
	var sock *httptest.Server
	up := websocket.Upgrader{}
	sock = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		_, msg, err := c.ReadMessage()
		if err != nil {
			return
		}
		watched <- string(msg)
		page := strings.TrimPrefix(string(msg), "watch ")
		c.WriteMessage(websocket.TextMessage, []byte("update\rhttps://other.example/elsewhere.md")) // not this page: ignored
		c.WriteMessage(websocket.TextMessage, []byte("update\r"+page))
		time.Sleep(time.Second)
	}))
	defer sock.Close()
	var site *httptest.Server
	site = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/bonbon.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"urlUpdateSocket":"` + strings.Replace(sock.URL, "http", "ws", 1) + `"}`))
		case "/a.md":
			w.Header().Set("Content-Type", "text/markdown")
			w.Write([]byte("# A\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer site.Close()

	b := New(history.Open(""), fetch.NewFetcher(nil, nil))
	var updates atomic.Int32
	got := make(chan string, 4)
	b.OnUpdate = func(tab int, u string) { updates.Add(1); got <- u }
	id := b.NewTab()

	b.Navigate(context.Background(), id, site.URL+"/a.md") // not in Bonbon mode: no socket
	select {
	case m := <-watched:
		t.Fatalf("watched outside Bonbon mode: %s", m)
	case <-time.After(300 * time.Millisecond):
	}

	b.SetBonbon(true)
	b.Navigate(context.Background(), id, site.URL+"/a.md")
	select {
	case m := <-watched:
		if m != "watch "+site.URL+"/a.md" {
			t.Fatalf("watch message = %q", m)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the page was not watched")
	}
	select {
	case u := <-got:
		if u != site.URL+"/a.md" {
			t.Fatalf("update for %q", u)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no update arrived")
	}
	if n := updates.Load(); n != 1 {
		t.Errorf("%d updates; the one for another page should be ignored", n)
	}
	b.CloseTab(id)

	// What happened on the socket is in the console.
	var seen []string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		seen = seen[:0]
		for _, e := range b.fetcher.Log.Entries() {
			if e.Proto == "websocket" {
				seen = append(seen, e.Method+" "+e.StatusText)
			}
		}
		if strings.Contains(strings.Join(seen, "|"), "WS CLOSE") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	consoleText := strings.Join(seen, "|")
	for _, want := range []string{"WS OPEN 101 Switching Protocols", "WS SEND watch " + site.URL + "/a.md", "WS RECV update " + site.URL + "/a.md", "WS CLOSE closed: the tab left the page"} {
		if !strings.Contains(consoleText, want) {
			t.Errorf("the console lacks %q; it has %s", want, consoleText)
		}
	}
}
