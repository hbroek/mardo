package browser

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html/charset"

	"mardo/internal/fetch"
)

// A site can publish a table of contents at /.well-known/toc.opml, an OPML outline whose lines link to its
// pages, newest first. A page that names no urlPrev or urlNext gets its arrows from it: the line after the page
// is the older page (the left arrow) and the line before it the newer one (the right). A folder can have a
// table of contents of its own, which is used for the pages under it. An outline can pull in another with an
// include node, from the same site only. Bonbon mode asks for it (see Browser.SetBonbon).
const (
	tocTTL          = 10 * time.Minute // how long an outline, or the news that there is none, is kept
	maxIncludeDepth = 3
	maxIncludes     = 20 // files one outline may pull in
	maxTocLevels    = 8  // folders looked at above a page
)

// tocTimeout is how long finding the arrows of a page may take altogether. A variable so a test can shorten it.
var tocTimeout = 5 * time.Second

type tocItem struct {
	entries []string // the addresses of the pages, in the order of the outline
	at      time.Time
}

type tocCache struct {
	mu sync.Mutex
	m  map[string]tocItem
}

type opmlOutline struct {
	Type     string        `xml:"type,attr"`
	URL      string        `xml:"url,attr"`
	Outlines []opmlOutline `xml:"outline"`
}

type opmlDoc struct {
	XMLName xml.Name      `xml:"opml"`
	Body    []opmlOutline `xml:"body>outline"`
}

// tocArrows gives the front matter values urlprev and urlnext for the page that was fetched, from the table of
// contents of its site, or nil when there is none that lists it.
func (b *Browser) tocArrows(ctx context.Context, res *fetch.Result, base *url.URL) map[string]string {
	if base == nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, tocTimeout)
	defer cancel()
	want := map[string]bool{tocKey(base.String()): true}
	if loc := res.Header.Get("Content-Location"); loc != "" {
		if u, err := base.Parse(loc); err == nil {
			want[tocKey(u.String())] = true
		}
	}
	for _, u := range wellKnownURLs(base, "toc.opml") {
		entries := b.tocEntries(ctx, u)
		for i, e := range entries {
			if want[tocKey(e)] {
				arrows := map[string]string{}
				if i > 0 {
					arrows["urlnext"] = entries[i-1] // newer
				}
				if i+1 < len(entries) {
					arrows["urlprev"] = entries[i+1] // older
				}
				return arrows
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil
}

// wellKnownURLs lists where the site file name (toc.opml, bonbon.json) may be for the page at base: in the folder of
// the page and in each folder above it, the nearest first.
func wellKnownURLs(base *url.URL, name string) []string {
	dir := path.Dir(base.Path)
	if strings.HasSuffix(base.Path, "/") {
		dir = strings.TrimSuffix(base.Path, "/")
	}
	var out []string
	for level := 0; level < maxTocLevels; level++ {
		d := *base
		d.Path, d.RawPath, d.RawQuery, d.Fragment = strings.TrimSuffix(dir, "/")+"/.well-known/"+name, "", "", ""
		out = append(out, d.String())
		if dir == "" || dir == "/" || dir == "." {
			break
		}
		dir = path.Dir(dir)
	}
	return out
}

// tocKey is how addresses are compared: without a fragment.
func tocKey(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	u.Fragment = ""
	return u.String()
}

// tocEntries is the list of pages in the outline at tocURL (includes followed), nil when there is none. It is
// kept for a while, which includes the answer "none".
func (b *Browser) tocEntries(ctx context.Context, tocURL string) []string {
	b.toc.mu.Lock()
	if it, ok := b.toc.m[tocURL]; ok && time.Since(it.at) < tocTTL {
		b.toc.mu.Unlock()
		return it.entries
	}
	b.toc.mu.Unlock()
	budget := maxIncludes
	entries := b.readTOC(ctx, tocURL, 0, &budget)
	if ctx.Err() != nil {
		return nil // too slow: ask again another time
	}
	b.toc.mu.Lock()
	if b.toc.m == nil {
		b.toc.m = map[string]tocItem{}
	}
	b.toc.m[tocURL] = tocItem{entries, time.Now()}
	b.toc.mu.Unlock()
	return entries
}

func (b *Browser) readTOC(ctx context.Context, tocURL string, depth int, budget *int) []string {
	res, err := b.fetcher.Get(ctx, tocURL)
	if err != nil || res.Status != http.StatusOK {
		return nil
	}
	dec := xml.NewDecoder(bytes.NewReader(res.Body))
	dec.CharsetReader = charset.NewReaderLabel
	var doc opmlDoc
	if err := dec.Decode(&doc); err != nil {
		return nil
	}
	base, err := url.Parse(res.URL)
	if err != nil {
		return nil
	}
	var out []string
	var walk func(outlines []opmlOutline)
	walk = func(outlines []opmlOutline) {
		for _, o := range outlines {
			switch {
			case o.Type == "include":
				u := resolve(base, o.URL)
				if u == nil || u.Scheme != base.Scheme || u.Host != base.Host || depth >= maxIncludeDepth || *budget <= 0 {
					continue // an outline from another site is not followed
				}
				*budget--
				out = append(out, b.readTOC(ctx, u.String(), depth+1, budget)...)
			case len(o.Outlines) > 0:
				walk(o.Outlines) // a month is not a page: its days are
			case o.URL != "":
				if u := resolve(base, o.URL); u != nil && (u.Scheme == "http" || u.Scheme == "https") {
					out = append(out, u.String())
				}
			}
		}
	}
	walk(doc.Body)
	return out
}

func resolve(base *url.URL, ref string) *url.URL {
	if ref == "" {
		return nil
	}
	u, err := base.Parse(ref)
	if err != nil {
		return nil
	}
	return u
}
