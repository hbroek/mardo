// Package render turns markdown into sanitized HTML and extracts links from HTML.
package render

import (
	"bytes"
	"html"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync/atomic"

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

// mdStrict is md for Bonbon mode: raw HTML in the text is left out instead of passed on.
var mdStrict = goldmark.New(
	goldmark.WithExtensions(extension.GFM, extension.Footnote),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

// Bonbon is Bonbon mode: a page is markdown only. HTML in the text is dropped, SVG pictures are links, a
// picture alone in a paragraph is centered with its alt text as a caption, and one at the head of a paragraph
// floats right. See the charter at bonbonweb.org.
var Bonbon atomic.Bool

var policy = newPolicy()

func newPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^[\w\-.:%]+$`)).OnElements("h1", "h2", "h3", "h4", "h5", "h6", "li", "sup")
	p.AllowAttrs("id", "name").Matching(regexp.MustCompile(`^[\w\-.:]+$`)).OnElements("a")
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
	return MarkdownWith(src, base, nil)
}

// HasArrows reports whether the front matter of src names urlPrev or urlNext.
func HasArrows(src []byte) bool {
	meta, _ := frontMatter(bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")))
	return meta["urlprev"] != "" || meta["urlnext"] != ""
}

// MarkdownWith is Markdown for a page whose front matter is given more values: defaults (urlprev and urlnext,
// from a table of contents) apply when the page names neither itself.
func MarkdownWith(src []byte, base *url.URL, defaults map[string]string) (string, string, error) {
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	meta, body := frontMatter(src)
	if len(defaults) > 0 && meta["urlprev"] == "" && meta["urlnext"] == "" {
		if meta == nil {
			meta = map[string]string{}
		}
		for k, v := range defaults {
			meta[k] = v
		}
	}
	strict := Bonbon.Load()
	conv := md
	if strict {
		conv = mdStrict
	}
	var buf bytes.Buffer
	if err := conv.Convert(expandMacros(body, meta), &buf); err != nil {
		return "", "", err
	}
	text := buf.Bytes()
	if head := bonbonHeader(meta); head != "" {
		text = append([]byte(head), text...)
	}
	out, title, err := finish(policy.SanitizeBytes(text), base, strict)
	if t := meta["title"]; t != "" {
		title = t
	}
	return out, title, err
}

// webAddress is s when it is a relative address or an http(s) one, else "": a front matter value must not
// become a javascript: link.
func webAddress(s string) string {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return s
}

// bonbonHeader is the HTML above the text for a page with Bonbon front matter: arrows to the previous and
// next page, and a right margin picture below them. It is "" for a page without those keys. The title,
// author and date are not drawn: a page puts them where it wants with {{page.title}} and the like. Everything
// is escaped, and the result goes through the same cleaning as the page.
func bonbonHeader(meta map[string]string) string {
	prev, next, img := webAddress(meta["urlprev"]), webAddress(meta["urlnext"]), webAddress(meta["rightmarginimage"])
	if prev == "" && next == "" && img == "" {
		return ""
	}
	e := html.EscapeString
	var b strings.Builder
	b.WriteString(`<div class="bonbon-head">`)
	if prev != "" || next != "" {
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
	if img != "" {
		b.WriteString(`<img class="bonbon-margin" src="` + e(img) + `" alt="">`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

var macro = regexp.MustCompile(`\{\{\s*page\.([A-Za-z0-9_.-]+)\s*\}\}`)

// expandMacros replaces {{page.title}} and the like in the text with the page's own front matter
// values. A macro that names a key the page does not have is left as written. When the page has a link,
// its title is a link to it: that is what link means.
func expandMacros(body []byte, meta map[string]string) []byte {
	if len(meta) == 0 || !bytes.Contains(body, []byte("{{")) {
		return body
	}
	return macro.ReplaceAllFunc(body, func(m []byte) []byte {
		key := strings.ToLower(string(macro.FindSubmatch(m)[1]))
		v, ok := meta[key]
		if !ok || v == "" {
			return m
		}
		if l := webAddress(meta["link"]); key == "title" && l != "" {
			v = "[" + strings.NewReplacer("[", `\[`, "]", `\]`).Replace(v) + "](<" + strings.ReplaceAll(l, " ", "%20") + ">)"
		}
		return []byte(v)
	})
}

// FrontMatter returns the front matter of a markdown document, with its keys in lower case (nested ones as
// source.url), and the text that follows it. A document without front matter gives no keys and itself.
func FrontMatter(src []byte) (map[string]string, []byte) {
	meta, body := frontMatter(bytes.TrimPrefix(src, []byte("\xef\xbb\xbf")))
	return meta, body
}

var frontKey = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*:(\s|$)`)

// frontChild is an indented key: line, the child of the key above it that has no value of its own.
var frontChild = regexp.MustCompile(`^[ \t]+[A-Za-z0-9_][A-Za-z0-9_-]*:(\s|$)`)

// frontParent is a key of a nested block that is still open: it has no value of its own, and what is
// indented under it is named after it (source: then url: is "source.url").
type frontParent struct {
	indent int
	key    string
}

func frontValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		v = v[1 : len(v)-1]
	}
	return v
}

// frontMatter splits a leading block between two --- lines off src when it reads as YAML metadata
// (key: value lines, with indented or list continuations); anything else, such as a document that
// starts with a horizontal rule, is left whole. The values of simple keys are returned, and of nested ones under their dotted names (source.url).
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
	var parents []frontParent
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
		case frontKey.MatchString(line), frontChild.MatchString(line):
			keys++
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			for len(parents) > 0 && parents[len(parents)-1].indent >= indent {
				parents = parents[:len(parents)-1]
			}
			k, v, _ := strings.Cut(strings.TrimSpace(line), ":")
			name := strings.ToLower(k)
			if len(parents) > 0 {
				name = parents[len(parents)-1].key + "." + name
			}
			v = frontValue(v)
			meta[name] = v
			if v == "" { // a nested block follows, or nothing
				parents = append(parents, frontParent{indent, name})
			}
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
	out, _, err := finish(policy.SanitizeBytes([]byte(src)), base, false)
	return out, err
}

// finish resolves links in already sanitized HTML and finds the first h1.
func finish(clean []byte, base *url.URL, strict bool) (string, string, error) {
	body := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Body, Data: "body"}
	nodes, err := xhtml.ParseFragment(bytes.NewReader(clean), body)
	if err != nil {
		return "", "", err
	}
	title := ""
	var out bytes.Buffer
	for _, n := range nodes {
		rewrite(n, base, &title, strict)
		if err := xhtml.Render(&out, n); err != nil {
			return "", "", err
		}
	}
	return out.String(), title, nil
}

// dropReservedNames removes an id or name that would stand in for one of the app's own globals. A page's id
// becomes a property of the window, and the app reads a few that only the host sets (mardoDocuments, mardoEmit,
// MardoAndroid, webkit), so a page must not be able to fill them in.
var reservedNames = map[string]bool{"mardodocuments": true, "mardoemit": true, "mardoreply": true, "mardoandroid": true, "webkit": true}

func dropReservedNames(n *xhtml.Node) {
	kept := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Key == "id" || a.Key == "name" {
			if reservedNames[strings.ToLower(a.Val)] {
				continue
			}
		}
		kept = append(kept, a)
	}
	n.Attr = kept
}

func rewrite(n *xhtml.Node, base *url.URL, title *string, strict bool) {
	if n.Type == xhtml.ElementNode {
		dropReservedNames(n)
		switch n.DataAtom {
		case atom.P:
			if strict {
				bonbonPicture(n)
			}
		case atom.A:
			resolveAttr(n, "href", base, false)
		case atom.Img:
			resolveAttr(n, "src", base, true)
			if strict {
				noSVG(n)
			}
		case atom.H1:
			if *title == "" {
				*title = collapse(text(n))
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		rewrite(c, base, title, strict)
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

// noSVG turns a picture that is an SVG into a link to it, with its alt text (or its name) for the text: Bonbon
// shows PNG, JPEG, GIF and WebP only.
func noSVG(n *xhtml.Node) {
	src, alt := attr(n, "src"), attr(n, "alt")
	u, err := url.Parse(src)
	if err != nil || (strings.ToLower(path.Ext(u.Path)) != ".svg" && !strings.Contains(strings.ToLower(u.RawQuery), "format=svg")) {
		return
	}
	if alt == "" {
		alt = path.Base(u.Path)
	}
	n.Type, n.Data, n.DataAtom = xhtml.ElementNode, "a", atom.A
	n.Attr = []xhtml.Attribute{{Key: "href", Val: src}}
	n.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: alt})
}

func attrOf(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// bonbonPicture applies Bonbon's two rules for pictures to a paragraph: one that is only a picture is marked as
// a figure, centered, with its alt text drawn under it; one that begins with a picture and goes on with text
// floats the picture at the right.
func bonbonPicture(p *xhtml.Node) {
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		if c.DataAtom == atom.Img {
			noSVG(c) // a picture that is not one stays out of the rules below
		}
	}
	var first *xhtml.Node
	others := 0
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == xhtml.TextNode && strings.TrimSpace(c.Data) == "":
		case first == nil && c.Type == xhtml.ElementNode && c.DataAtom == atom.Img:
			first = c
		default:
			others++
		}
	}
	if first == nil {
		return
	}
	if others > 0 {
		first.Attr = append(first.Attr, xhtml.Attribute{Key: "class", Val: "bonbon-right"})
		return
	}
	// The paragraph stays a paragraph with its picture alone in it, which is what a stylesheet's p > img:only-child
	// takes for a standalone picture; the alt text is the caption, which the style draws from data-caption.
	p.Attr = []xhtml.Attribute{{Key: "class", Val: "bonbon-figure"}}
	if alt := strings.TrimSpace(attrOf(first, "alt")); alt != "" {
		p.Attr = append(p.Attr, xhtml.Attribute{Key: "data-caption", Val: alt})
	}
}
