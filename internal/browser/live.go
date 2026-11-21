package browser

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"mardo/internal/page"
)

// Live updates, as Bonbon has them: a site can name a websocket in its bonbon.json (urlUpdateSocket). The tab opens it,
// sends "watch " and the address of the page it shows, and when that page is rebuilt the site sends "update", a
// return and the address. The page is then loaded again, by whoever listens to OnUpdate. One socket per tab and one
// watch per socket; a socket that drops is opened again after a while, as long as the tab still shows the page.
// Only in Bonbon mode.

const (
	updatePrefix       = "update\r"
	updateRetry        = 30 * time.Second // how long a tab waits before opening its socket again after it dropped
	updateDialTimeout  = 10 * time.Second
	updatePingEvery    = 2 * time.Minute
	updateQuietTimeout = 5 * time.Minute // a socket that does not answer a ping in this time is taken for dead
)

type liveWatch struct {
	url    string
	cancel context.CancelFunc
}

// watch starts, keeps or stops the update socket of a tab, for the page it now shows. t.mu is held.
func (b *Browser) watch(tabID int, t *tab, p page.Page) {
	if p.Kind == page.KindError { // the tab still shows what it showed
		return
	}
	want := ""
	if b.Bonbon() && b.OnUpdate != nil && p.Kind == page.KindMarkdown && !isSearchURL(p.URL) {
		if u, err := url.Parse(p.URL); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
			u.Fragment = ""
			want = u.String()
		}
	}
	if t.live != nil {
		if t.live.url == want {
			return
		}
		t.live.cancel()
		t.live = nil
	}
	if want == "" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.live = &liveWatch{url: want, cancel: cancel}
	go b.follow(ctx, tabID, want)
}

// follow keeps a socket open for the page until ctx ends or the site has none.
func (b *Browser) follow(ctx context.Context, tabID int, pageURL string) {
	for ctx.Err() == nil {
		cfg := b.siteConfig(ctx, pageURL)
		if cfg == nil || cfg.URLUpdateSocket == "" {
			return
		}
		sock := socketAddress(cfg)
		if sock == "" {
			return
		}
		b.listen(ctx, sock, tabID, pageURL)
		if ctx.Err() == nil {
			b.fetcher.Log.Socket("WS RETRY", sock, "opening it again in "+updateRetry.String(), nil)
		}
		select {
		case <-ctx.Done():
		case <-time.After(updateRetry):
		}
	}
}

// socketAddress is the websocket of a site's bonbon.json: a ws or wss address, or one relative to the file.
func socketAddress(cfg *SiteConfig) string {
	u, err := url.Parse(strings.TrimSpace(cfg.URLUpdateSocket))
	if err != nil {
		return ""
	}
	if base, err := url.Parse(cfg.url); err == nil {
		u = base.ResolveReference(u)
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return ""
	}
	return u.String()
}

// listen watches one page on one connection until it drops or ctx ends.
func (b *Browser) listen(ctx context.Context, sock string, tabID int, pageURL string) {
	log := b.fetcher.Log
	dialer := websocket.Dialer{HandshakeTimeout: updateDialTimeout, Proxy: http.ProxyFromEnvironment}
	conn, _, err := dialer.DialContext(ctx, sock, nil)
	if err != nil {
		if ctx.Err() == nil {
			log.Socket("WS OPEN", sock, "", err)
		}
		return
	}
	log.Socket("WS OPEN", sock, "", nil)
	defer conn.Close()
	go func() { // the tab left the page, or closed: the read below stops
		<-ctx.Done()
		conn.Close()
	}()
	if err := conn.WriteMessage(websocket.TextMessage, []byte("watch "+pageURL)); err != nil {
		log.Socket("WS SEND", sock, "watch "+pageURL, err)
		return
	}
	log.Socket("WS SEND", sock, "watch "+pageURL, nil)
	conn.SetReadDeadline(time.Now().Add(updateQuietTimeout))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(updateQuietTimeout)) })
	go func() {
		tick := time.NewTicker(updatePingEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)) != nil {
					return
				}
			}
		}
	}()
	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				log.Socket("WS CLOSE", sock, "closed: the tab left the page", nil)
			} else {
				log.Socket("WS CLOSE", sock, "", err)
			}
			return
		}
		conn.SetReadDeadline(time.Now().Add(updateQuietTimeout))
		if kind != websocket.TextMessage {
			continue
		}
		log.Socket("WS RECV", sock, shortMessage(string(data)), nil)
		if got, ok := strings.CutPrefix(string(data), updatePrefix); ok && ctx.Err() == nil && b.OnUpdate != nil && strings.TrimSpace(got) == pageURL {
			b.OnUpdate(tabID, pageURL)
		}
	}
}

// shortMessage is a message for the console on one line, cut short.
func shortMessage(s string) string {
	s = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(s)
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300]) + "…"
	}
	return s
}
