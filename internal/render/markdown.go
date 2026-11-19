// Package render turns markdown into sanitized HTML and extracts links from HTML.
package render

import (
	"bytes"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mardo/internal/fetch"
	"mardo/internal/imgproxy"
	"mardo/internal/localfile"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	ghtml "github.com/yuin/goldmark/renderer/html"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	// Raw HTML in markdown is passed on, and cleaned by the policy below like everything else.
	goldmark.WithRendererOptions(ghtml.WithUnsafe()),
)

var policy = newPolicy()

func newPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^[\w\-.:%]+$`)).OnElements("h1", "h2", "h3", "h4", "h5", "h6", "li", "sup")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("checked", "disabled").OnElements("input")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^bonbon[\w-]*$`)).OnElements("div", "a", "img", "p", "h1", "span")
	return p
}

// Markdown renders src to sanitized HTML. Relative links and image sources are
// resolved against base. It returns the HTML and the text of the first h1.
//
// A block of front matter at the start (--- key: value ---) is not part of the page: it is
// left out. Its title names the page (and the tab), ahead of the first h1. Bonbon keys (link,
// author, pubDate, urlPrev, urlNext, rightMarginImage) add a header above the text, see bonbonHeader.
func Markdown(src []byte, base *url.URL) (string, string, error) {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	meta, body := frontMatter(src)
	var buf bytes.Buffer
	if err := md.Convert(expandMacros(body, meta), &buf); err != nil {
		return "", "", err
	}
	text := buf.Bytes()
	if head := bonbonHeader(meta, bytes.Contains(text, []byte("<h1"))); head != "" {
		text = append([]byte(head), text...)
	}
	out, title, err := finish(policy.SanitizeBytes(text), base)
	if t := meta["title"]; t != "" {
		title = t
	}
	return out, title, err
}

// bonbonHeader is the HTML above the text for a page with Bonbon front matter: arrows to the previous and
// next page, a right margin picture below them, the title (a link when link is set) and a line with the author and publication date. It is "" for a page without any
// of those keys, so ordinary front matter (a title and a description) shows nothing extra. The
// title is not repeated when the text has its own h1. Everything is escaped, and the result goes
// through the same cleaning as the page.
func bonbonHeader(meta map[string]string, hasH1 bool) string {
	if meta["link"] == "" && meta["author"] == "" && meta["pubdate"] == "" && meta["urlprev"] == "" && meta["urlnext"] == "" && meta["rightmarginimage"] == "" {
		return ""
	}
	e := html.EscapeString
	var b strings.Builder
	b.WriteString(`<div class="bonbon-head">`)
	if prev, next := meta["urlprev"], meta["urlnext"]; prev != "" || next != "" {
		b.WriteString(`<div class="bonbon-arrows">`)
		if prev != "" {
			b.WriteString(`<a class="bonbon-prev" href="` + e(prev) + `" title="Previous" aria-label="Previous">←</a>`)
		}
		if next != "" {
			b.WriteString(`<a class="bonbon-next" href="` + e(next) + `" title="Next" aria-label="Next">→</a>`)
		}
		b.WriteString(`</div>`)
	}
	// after the arrows, so the picture starts below them
	if img := meta["rightmarginimage"]; img != "" {
		b.WriteString(`<img class="bonbon-margin" src="` + e(img) + `" alt="">`)
	}
	if t := meta["title"]; t != "" && !hasH1 {
		if l := meta["link"]; l != "" {
			t = `<a href="` + e(l) + `">` + e(t) + `</a>`
		} else {
			t = e(t)
		}
		b.WriteString(`<h1 class="bonbon-title">` + t + `</h1>`)
	}
	var by []string
	if a := meta["author"]; a != "" {
		by = append(by, e(a))
	}
	if d := meta["pubdate"]; d != "" {
		by = append(by, e(pubDate(d)))
	}
	if len(by) > 0 {
		b.WriteString(`<p class="bonbon-byline">` + strings.Join(by, " · ") + `</p>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// pubDate shows an RSS 2.0 date (Fri, 23 Oct 2026 15:19:45 GMT) as "23 Oct 2026". A date that
// does not read that way is shown as written.
func pubDate(s string) string {
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 MST", "Mon, 2 Jan 2006 15:04:05 -0700", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2 Jan 2006")
		}
	}
	return s
}

var macro = regexp.MustCompile(`\{\{\s*page\.([A-Za-z0-9_-]+)\s*\}\}`)

// expandMacros replaces {{page.title}} and the like in the text with the page's own front matter
// values. A macro that names a key the page does not have is left as written.
func expandMacros(body []byte, meta map[string]string) []byte {
	if len(meta) == 0 || !bytes.Contains(body, []byte("{{")) {
		return body
	}
	return macro.ReplaceAllFunc(body, func(m []byte) []byte {
		if v, ok := meta[strings.ToLower(string(macro.FindSubmatch(m)[1]))]; ok && v != "" {
			return []byte(v)
		}
		return m
	})
}

var frontKey = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*:(\s|$)`)

// frontMatter splits a leading block between two --- lines off src when it reads as YAML metadata
// (key: value lines, with indented or list continuations); anything else, such as a document that
// starts with a horizontal rule, is left whole. The values of simple keys are returned.
func frontMatter(src []byte) (map[string]string, []byte) {
	text := string(src)
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		if rest, ok = strings.CutPrefix(text, "---\r\n"); !ok {
			return nil, src
		}
	}
	meta := map[string]string{}
	consumed := len(text) - len(rest)
	keys := 0
	for len(rest) > 0 {
		line, tail, _ := strings.Cut(rest, "\n")
		consumed += len(rest) - len(tail)
		rest = tail
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.TrimSpace(line) == "---" || strings.TrimSpace(line) == "...":
			if keys == 0 {
				return nil, src
			}
			return meta, src[consumed:]
		case frontKey.MatchString(line):
			keys++
			k, v, _ := strings.Cut(line, ":")
			v = strings.TrimSpace(v)
			if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
				v = v[1 : len(v)-1]
			}
			meta[strings.ToLower(k)] = v
		case strings.TrimSpace(line) == "", strings.HasPrefix(line, " "), strings.HasPrefix(line, "\t"), strings.HasPrefix(line, "- "), strings.HasPrefix(line, "#"):
			// a blank line, a continuation of the previous key, or a comment
		default:
			return nil, src // not metadata
		}
	}
	return nil, src // never closed
}

// SanitizeHTML cleans untrusted HTML (such as a feed item body) and resolves
// its links against base.
func SanitizeHTML(src string, base *url.URL) (string, error) {
	out, _, err := finish(policy.SanitizeBytes([]byte(src)), base)
	return out, err
}

// finish resolves links in already sanitized HTML and finds the first h1.
func finish(clean []byte, base *url.URL) (string, string, error) {
	body := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Body, Data: "body"}
	nodes, err := xhtml.ParseFragment(bytes.NewReader(clean), body)
	if err != nil {
		return "", "", err
	}
	title := ""
	var out bytes.Buffer
	for _, n := range nodes {
		rewrite(n, base, &title)
		if err := xhtml.Render(&out, n); err != nil {
			return "", "", err
		}
	}
	return out.String(), title, nil
}

func rewrite(n *xhtml.Node, base *url.URL, title *string) {
	if n.Type == xhtml.ElementNode {
		switch n.DataAtom {
		case atom.A:
			resolveAttr(n, "href", base, false)
		case atom.Img:
			resolveAttr(n, "src", base, true)
		case atom.H1:
			if *title == "" {
				*title = collapse(text(n))
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		rewrite(c, base, title)
	}
}

// resolveAttr makes a relative URL absolute. Fragment-only hrefs are kept so
// the frontend can scroll in place. Images are limited to http and https,
// except that a page opened from disk may show images from disk, which are
// routed through the local file handler.
func resolveAttr(n *xhtml.Node, key string, base *url.URL, isImage bool) {
	if base == nil {
		base = &url.URL{} // a page with no address: relative links stay as they are
	}
	for i, a := range n.Attr {
		if a.Key != key {
			continue
		}
		if strings.HasPrefix(a.Val, "#") {
			return
		}
		ref, err := url.Parse(a.Val)
		if err != nil {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
			return
		}
		abs := base.ResolveReference(ref)
		switch {
		case !isImage:
			n.Attr[i].Val = abs.String()
		case imgproxy.Wanted(abs):
			n.Attr[i].Val = imgproxy.URL(abs.String())
		case abs.Scheme == "http" || abs.Scheme == "https":
			n.Attr[i].Val = abs.String()
		case abs.Scheme == "file" && base.Scheme == "file":
			n.Attr[i].Val = localfile.URL(fetch.FilePath(abs))
		default:
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
		}
		return
	}
}

func text(n *xhtml.Node) string {
	if n.Type == xhtml.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(text(c))
	}
	return sb.String()
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
