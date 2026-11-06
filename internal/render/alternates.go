package render

import (
	"bytes"
	"mime"
	"net/url"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
)

// Alternate is another version of a web page that the page itself points to:
// <link rel="alternate" type="application/rss+xml" href="/rss/">.
type Alternate struct {
	Type string // media type, lower case, without parameters
	Href string // absolute address
}

// Alternates lists the alternate versions a web page advertises in its head, in the order given.
func Alternates(body []byte, contentType string, base *url.URL) []Alternate {
	r, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return nil
	}
	doc, err := xhtml.Parse(r)
	if err != nil {
		return nil
	}
	var out []Alternate
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.DataAtom == atom.Link {
			var rel, typ, href string
			for _, a := range n.Attr {
				switch a.Key {
				case "rel":
					rel = a.Val
				case "type":
					typ = a.Val
				case "href":
					href = a.Val
				}
			}
			mt, _, err := mime.ParseMediaType(typ)
			if err == nil && href != "" && hasWord(rel, "alternate") {
				if u, err := base.Parse(strings.TrimSpace(href)); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
					out = append(out, Alternate{Type: mt, Href: u.String()})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

func hasWord(list, word string) bool {
	for _, w := range strings.Fields(strings.ToLower(list)) {
		if w == word {
			return true
		}
	}
	return false
}

// IsFeedType reports whether a media type is an RSS or Atom feed.
func IsFeedType(mt string) bool {
	return mt == "application/rss+xml" || mt == "application/atom+xml"
}
