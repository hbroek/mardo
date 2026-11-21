package core

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// The pictures of web addresses that actions use as their icons (icon: favicon, or the address of a picture) are kept
// in the config folder, in favicons.json, so that they are there again after a restart. One is looked at again after
// a week, and meanwhile the old one is shown; an address that did not answer is tried again after an hour.

const (
	iconFresh    = 7 * 24 * time.Hour
	iconRetry    = time.Hour
	iconFileName = "favicons.json"
)

type iconEntry struct {
	Data string    `json:"data"` // a data address; "" for an address that did not answer
	At   time.Time `json:"at"`
}

func (a *Core) iconFile() string {
	if a.dir == "" {
		return ""
	}
	return filepath.Join(a.dir, iconFileName)
}

// loadIcons reads the kept pictures.
func (a *Core) loadIcons() {
	a.iconMu.Lock()
	defer a.iconMu.Unlock()
	a.icons = map[string]iconEntry{}
	if f := a.iconFile(); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(data, &a.icons)
		}
	}
}

// saveIcons writes the pictures that were got. a.iconMu is held.
func (a *Core) saveIcons() {
	f := a.iconFile()
	if f == "" {
		return
	}
	keep := map[string]iconEntry{}
	for k, e := range a.icons {
		if e.Data != "" {
			keep[k] = e
		}
	}
	data, err := json.Marshal(keep)
	if err != nil {
		return
	}
	tmp := f + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, f)
	}
}

// webIcon is the picture at a web address, as a data address, when it has been got; "" until then, and then the
// frontend is told (actionicons) to look again.
func (a *Core) webIcon(address string) string {
	a.iconMu.Lock()
	defer a.iconMu.Unlock()
	if a.icons == nil {
		a.icons = map[string]iconEntry{}
	}
	if a.iconsOff {
		return a.icons[address].Data
	}
	e, known := a.icons[address]
	age := time.Since(e.At)
	switch {
	case known && e.Data != "" && age < iconFresh:
		return e.Data
	case known && e.Data == "" && age < iconRetry:
		return ""
	}
	if a.iconBusy == nil {
		a.iconBusy = map[string]bool{}
	}
	if !a.iconBusy[address] {
		a.iconBusy[address] = true
		a.iconWG.Add(1)
		go func() {
			defer a.iconWG.Done()
			a.fetchIcon(address)
		}()
	}
	return e.Data // an old one is better than none
}

// faviconPrefix marks the icon of a site, "favicon:https://example.com", as against the address of a picture.
const faviconPrefix = "favicon:"

// favdirPrefix marks the icon of the pages of a site for the address bar, "favdir:https://example.com" or, for a site
// that keeps others in folders, "favdir:https://example.com/folder". See pageIconCandidates.
const favdirPrefix = "favdir:"

// pageIconCandidates is where to look for the icon of a page of a site that has no <link rel="icon"> to say, which is
// what a markdown site is: the root of the domain, then the first folder of the page (a site that keeps others in
// folders), then what the home page names, if it is a web page. site is the origin, with that folder if there is one.
func (a *Core) pageIconCandidates(ctx context.Context, site string) []string {
	u, err := url.Parse(site)
	if err != nil {
		return nil
	}
	origin := u.Scheme + "://" + u.Host
	out := []string{origin + "/favicon.ico"}
	if u.Path != "" && u.Path != "/" {
		out = append(out, strings.TrimRight(site, "/")+"/favicon.ico")
	}
	return append(out, a.siteIcons(ctx, origin)...)
}

// fetchIcon gets a picture for the menu and keeps it.
func (a *Core) fetchIcon(address string) {
	data := ""
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	candidates := []string{address}
	if origin, ok := strings.CutPrefix(address, faviconPrefix); ok {
		candidates = append(a.siteIcons(ctx, origin), origin+"/favicon.ico")
	} else if site, ok := strings.CutPrefix(address, favdirPrefix); ok {
		candidates = a.pageIconCandidates(ctx, site)
	}
	for _, c := range candidates {
		if res, err := a.fetcher.GetImage(ctx, c); err == nil && res.Status == http.StatusOK && len(res.Body) <= maxActionIcon {
			if data = imageDataURL(c, res.ContentType, res.Body); data != "" {
				break
			}
		}
	}
	a.iconMu.Lock()
	delete(a.iconBusy, address)
	old := a.icons[address]
	switch {
	case data != "":
		a.icons[address] = iconEntry{Data: data, At: time.Now()}
		a.saveIcons()
	case old.Data != "": // it did not answer now: keep the old one, and look again in an hour
		a.icons[address] = iconEntry{Data: old.Data, At: time.Now().Add(iconRetry - iconFresh)}
	default:
		a.icons[address] = iconEntry{At: time.Now()}
	}
	changed := data != ""
	a.iconMu.Unlock()
	if changed {
		a.host.Emit("actionicons", nil)
	}
}

// siteIcons are the pictures a site names in the head of its home page (<link rel="icon">), the way a browser finds
// the icon of a site, best for a menu first: an SVG, then a PNG near 32 pixels, then an icon file, then the large
// pictures made for a phone's home screen. Not every site has a /favicon.ico (chatgpt.com has none).
func (a *Core) siteIcons(ctx context.Context, origin string) []string {
	base, err := url.Parse(origin + "/")
	if err != nil {
		return nil
	}
	res, err := a.fetcher.GetHTML(ctx, base.String())
	if err != nil || res.Status != http.StatusOK {
		return nil
	}
	return pageIcons(res.Body, base)
}

// pageIcons lists the icons that the head of a web page names, best first, as full addresses.
func pageIcons(body []byte, base *url.URL) []string {
	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}
	type found struct {
		href  string
		score int
	}
	var list []found
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.DataAtom == atom.Link {
			var rel, href, typ, sizes string
			for _, at := range n.Attr {
				switch at.Key {
				case "rel":
					rel = strings.ToLower(at.Val)
				case "href":
					href = strings.TrimSpace(at.Val)
				case "type":
					typ = strings.ToLower(at.Val)
				case "sizes":
					sizes = strings.ToLower(at.Val)
				}
			}
			isIcon, isTouch := false, false
			for _, w := range strings.Fields(rel) {
				switch w {
				case "icon":
					isIcon = true
				case "apple-touch-icon", "apple-touch-icon-precomposed":
					isTouch = true
				}
			}
			if u, err := base.Parse(href); err == nil && href != "" && (isIcon || isTouch) && (u.Scheme == "http" || u.Scheme == "https") {
				score := 50
				switch {
				case isTouch:
					score = 200
				case strings.Contains(typ, "svg") || strings.HasSuffix(strings.ToLower(u.Path), ".svg"):
					score = 0
				case sizes != "" && sizes != "any":
					if w, _, ok := strings.Cut(sizes, "x"); ok {
						if n, err := strconv.Atoi(w); err == nil {
							score = 10 + abs(n-32)
						}
					}
				}
				list = append(list, found{u.String(), score})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	sort.SliceStable(list, func(i, j int) bool { return list[i].score < list[j].score })
	var out []string
	for _, f := range list {
		out = append(out, f.href)
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
