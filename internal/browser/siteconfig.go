package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"

	"mardo/internal/fetch"
)

// A site can describe itself in /.well-known/bonbon.json (a folder can have its own, beside its own toc.opml). Every
// item is optional. Bonbon mode reads it for the search of the site; the other items are read but not used yet.
type SiteConfig struct {
	URLSearch       string `json:"urlSearch"`       // the site's search: ?q= and the words are added, a markdown page is expected back
	URLUpdateSocket string `json:"urlUpdateSocket"` // a websocket that says when a page changed
	URLMirrorOf     string `json:"urlMirrorOf"`     // the HTML site this one is the markdown version of
	CName           string `json:"cname"`           // the name the owner would like for a site served from a repository

	url string // where it was read from; relative addresses in it are relative to it
}

type siteConfigItem struct {
	cfg *SiteConfig
	at  time.Time
}

type siteConfigCache struct {
	mu sync.Mutex
	m  map[string]siteConfigItem
}

// siteConfig is the bonbon.json that applies to the page at pageURL: the nearest folder's that has one. nil when
// there is none. Each answer, "none" too, is kept for a while.
func (b *Browser) siteConfig(ctx context.Context, pageURL string) *SiteConfig {
	base, err := url.Parse(pageURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, tocTimeout)
	defer cancel()
	for _, u := range wellKnownURLs(base, "bonbon.json") {
		if cfg := b.readSiteConfig(ctx, u); cfg != nil {
			return cfg
		}
		if ctx.Err() != nil {
			return nil
		}
	}
	return nil
}

func (b *Browser) readSiteConfig(ctx context.Context, u string) *SiteConfig {
	b.siteCfg.mu.Lock()
	if it, ok := b.siteCfg.m[u]; ok && time.Since(it.at) < tocTTL {
		b.siteCfg.mu.Unlock()
		return it.cfg
	}
	b.siteCfg.mu.Unlock()
	var cfg *SiteConfig
	if res, err := b.fetcher.Get(ctx, u); err == nil && res.Status == http.StatusOK {
		var c SiteConfig
		if json.Unmarshal(res.Body, &c) == nil {
			c.url = res.URL
			cfg = &c
		}
	}
	if ctx.Err() != nil {
		return nil // too slow: ask again another time
	}
	b.siteCfg.mu.Lock()
	if b.siteCfg.m == nil {
		b.siteCfg.m = map[string]siteConfigItem{}
	}
	b.siteCfg.m[u] = siteConfigItem{cfg, time.Now()}
	b.siteCfg.mu.Unlock()
	return cfg
}

// searchEndpoint is the address of the site's search, or "" when the site has none. It may be on another host
// than the site, as Bonbon's own sites have it; it is an http or https address.
func (c *SiteConfig) searchEndpoint() string {
	if c == nil || c.URLSearch == "" {
		return ""
	}
	base, err := url.Parse(c.url)
	if err != nil {
		return ""
	}
	u, err := base.Parse(c.URLSearch)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}

// mirrorCandidates are the addresses on this site of the page that link, on the HTML site this one mirrors
// (urlMirrorOf), stands for, best first. The path under the mirror's address is looked for under the folder of
// the site: /2026/11/18.html is 2026/11/18.md, a folder is its index.md. nil when link is not on the mirrored
// site or has no such twin.
func (c *SiteConfig) mirrorCandidates(link *url.URL) []string {
	if c == nil || c.URLMirrorOf == "" {
		return nil
	}
	m, err := url.Parse(c.URLMirrorOf)
	if err != nil || m.Host == "" || !strings.EqualFold(m.Hostname(), link.Hostname()) {
		return nil
	}
	root := strings.TrimSuffix(m.Path, "/") + "/"
	if !strings.HasPrefix(link.Path+"/", root) && !strings.HasPrefix(link.Path, root) {
		return nil
	}
	rel := strings.TrimPrefix(link.Path, root)
	var twins []string
	switch ext := strings.ToLower(path.Ext(rel)); {
	case rel == "" || strings.HasSuffix(rel, "/"):
		twins = []string{rel + "index.md"}
	case ext == ".html" || ext == ".htm":
		twins = []string{strings.TrimSuffix(rel, path.Ext(rel)) + ".md"}
	case ext == "":
		twins = []string{rel + ".md", rel + "/index.md"}
	default:
		return nil
	}
	cu, err := url.Parse(c.url)
	if err != nil {
		return nil
	}
	folder := strings.TrimSuffix(cu.Path, ".well-known/bonbon.json")
	var out []string
	for _, t := range twins {
		u := *cu
		u.Path, u.RawPath, u.RawQuery, u.Fragment = folder+t, "", "", ""
		out = append(out, u.String())
	}
	return out
}

// mirrorOf is the page on the current page's own site that a link to the site it mirrors stands for: the address
// of its markdown version, or "" when the site says nothing of the kind or has no such page.
func (b *Browser) mirrorOf(ctx context.Context, current, link string) string {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || current == "" {
		return ""
	}
	candidates := b.siteConfig(ctx, current).mirrorCandidates(u)
	if len(candidates) == 0 {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, tocTimeout)
	defer cancel()
	for _, c := range candidates {
		if res, err := b.fetcher.Get(ctx, c); err == nil && res.Status == http.StatusOK && fetch.IsBonbonMarkdown(res.ContentType, res.URL) {
			return c
		}
	}
	return ""
}

var cnameHost = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)+$`)

// ownAddress is the address of the page at base under the name the site's owner would like for it (cname), for a
// site that is read at another address, such as a repository's. "" when the site names none, when this is it, or when
// the page is not under the folder that has the bonbon.json. It is only offered, never shown as the page's address,
// so a site cannot pass for another.
func (c *SiteConfig) ownAddress(base *url.URL) string {
	if c == nil || !cnameHost.MatchString(c.CName) || len(c.CName) > 253 || strings.EqualFold(c.CName, base.Hostname()) {
		return ""
	}
	cu, err := url.Parse(c.url)
	if err != nil {
		return ""
	}
	folder := strings.TrimSuffix(cu.Path, ".well-known/bonbon.json")
	if !strings.HasPrefix(base.Path, folder) {
		return ""
	}
	u := url.URL{Scheme: "https", Host: strings.ToLower(c.CName), Path: "/" + strings.TrimPrefix(base.Path, folder), RawQuery: base.RawQuery}
	return u.String()
}
