package core

import (
	"net/url"
	"strings"
)

// Favicon is the icon of a page for the address bar, as a data address; "" when there is none yet. Looked for in this
// order: the favicon: of the page's front matter (hint, a full address), the root of the domain, the first folder of
// the page, and what the home page names. The answer comes from the cache; when there is none the icon is looked for
// in the background and the frontend is told (actionicons) to ask again. It is kept per site (and per first folder).
func (a *Core) Favicon(pageURL, hint string) string {
	u, err := url.Parse(pageURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	if h := strings.TrimSpace(hint); h != "" {
		if strings.HasPrefix(h, "data:image/") && len(h) <= 2*maxActionIcon {
			return h
		}
		if hu, err := url.Parse(h); err == nil && (hu.Scheme == "http" || hu.Scheme == "https") {
			if d := a.webIcon(hu.String()); d != "" {
				return d
			}
			// The page names an icon that is not got yet (or never): the site's own is shown meanwhile.
		}
	}
	key := u.Scheme + "://" + u.Host
	if seg, _, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/"); seg != "" && strings.Contains(strings.TrimPrefix(u.Path, "/"), "/") {
		key += "/" + url.PathEscape(seg) // the first folder, when the page is in one
	}
	return a.webIcon(favdirPrefix + key)
}
