package render

import (
	"bytes"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
)

// ToMarkdown turns an HTML document into markdown with fixed rules: headings, paragraphs, emphasis, links,
// images, lists, quotes, code, rules and tables. By default it keeps the main content (the <main> or
// <article> element, else the body without its navigation, header, footer and sidebars); wholePage keeps
// everything. Scripts, styles, forms and hidden parts are always left out.
func ToMarkdown(body []byte, contentType string, base *url.URL, wholePage bool) (title, md string) {
	doc, ok := parseHTML(body, contentType)
	if !ok {
		return "", ""
	}
	title = documentTitle(doc)
	root := conversionRoot(doc, wholePage)
	c := &conv{base: base, chrome: !wholePage, header: !wholePage && root.DataAtom == atom.Body}
	md = tidy(c.blocks(root))
	if c.header && len(md) < 200 { // the page keeps its content in a header or a sidebar element: take it all
		if all := tidy((&conv{base: base}).blocks(conversionRoot(doc, true))); len(all) > len(md) {
			md = all
		}
	}
	if title == "" {
		title = firstHeading(doc)
	}
	if md != "" && !strings.HasPrefix(md, "# ") && title != "" {
		md = "# " + escapeInline(title) + "\n\n" + md
	}
	if md == "" {
		return title, ""
	}
	return title, md + "\n"
}

// SimplifiedHTML is the same content as ToMarkdown reads, reduced to its structure: the tags that carry meaning,
// with only the addresses and alternative texts left of their attributes. It is what a model is asked to convert,
// much shorter than the page and with the scripts, styles and wrappers gone.
func SimplifiedHTML(body []byte, contentType string, base *url.URL, wholePage bool) (title, simplified string) {
	doc, ok := parseHTML(body, contentType)
	if !ok {
		return "", ""
	}
	root := conversionRoot(doc, wholePage)
	var sb strings.Builder
	c := &conv{base: base, chrome: !wholePage, header: !wholePage && root.DataAtom == atom.Body}
	c.simplify(&sb, root)
	return documentTitle(doc), strings.TrimSpace(sb.String())
}

func parseHTML(body []byte, contentType string) (*xhtml.Node, bool) {
	r, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return nil, false
	}
	doc, err := xhtml.Parse(r)
	if err != nil {
		return nil, false
	}
	return doc, true
}

func documentTitle(doc *xhtml.Node) string {
	var title string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if title != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.DataAtom == atom.Title {
			title = collapse(text(n))
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	return title
}

func firstHeading(doc *xhtml.Node) string {
	var found string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if found != "" {
			return
		}
		if n.Type == xhtml.ElementNode && n.DataAtom == atom.H1 {
			found = collapse(text(n))
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	return found
}

// conversionRoot is where the content starts: the main element when the page has one.
func conversionRoot(doc *xhtml.Node, wholePage bool) *xhtml.Node {
	var body, main *xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			switch {
			case n.DataAtom == atom.Body && body == nil:
				body = n
			case main == nil && (n.DataAtom == atom.Main || attr(n, "role") == "main"):
				main = n
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	if !wholePage && main != nil {
		return main
	}
	if !wholePage && body != nil {
		// a page without <main> often has a single <article>
		if arts := elements(body, atom.Article); len(arts) == 1 {
			return arts[0]
		}
	}
	if body != nil {
		return body
	}
	return doc
}

func elements(n *xhtml.Node, a atom.Atom) []*xhtml.Node {
	var out []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.DataAtom == a {
			out = append(out, n)
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return out
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

type conv struct {
	base   *url.URL
	chrome bool // leave out navigation, footers and sidebars, wherever they are
	header bool // and headers, when the content is the whole body: an article's own header holds its title
}

// skip is true for what never belongs in the text of a page.
func (c *conv) skip(n *xhtml.Node) bool {
	if n.Type == xhtml.CommentNode || n.Type == xhtml.DoctypeNode {
		return true
	}
	if n.Type != xhtml.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Noscript, atom.Svg, atom.Template, atom.Iframe, atom.Head, atom.Title,
		atom.Button, atom.Select, atom.Textarea, atom.Canvas, atom.Object, atom.Embed, atom.Audio, atom.Video, atom.Dialog, atom.Math:
		return true
	case atom.Input:
		return attr(n, "type") != "checkbox" || !inListItem(n) // other checkboxes are menu switches
	case atom.Nav, atom.Aside, atom.Footer:
		return c.chrome
	case atom.Header:
		return c.header
	case atom.Form:
		return c.chrome
	}
	if c.chrome {
		switch attr(n, "role") {
		case "navigation", "banner", "contentinfo", "complementary", "search":
			return true
		}
	}
	return hidden(n)
}

func inListItem(n *xhtml.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == xhtml.ElementNode && p.DataAtom == atom.Li {
			return true
		}
	}
	return false
}

// isBlockEl are the elements that start a new block rather than continue a paragraph.
func isBlockEl(n *xhtml.Node) bool {
	switch n.DataAtom {
	case atom.P, atom.Div, atom.Section, atom.Article, atom.Main, atom.Header, atom.Footer, atom.Nav, atom.Aside,
		atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Ul, atom.Ol, atom.Dl, atom.Blockquote, atom.Pre, atom.Hr,
		atom.Table, atom.Figure, atom.Figcaption, atom.Form, atom.Fieldset, atom.Address, atom.Details, atom.Center, atom.Body, atom.Html,
		atom.Li, atom.Dt, atom.Dd, atom.Tr, atom.Thead, atom.Tbody, atom.Tfoot, atom.Caption, atom.Summary, atom.Hgroup:
		return true
	}
	return false
}

// blocks converts the children of parent into blocks separated by blank lines.
func (c *conv) blocks(parent *xhtml.Node) string {
	return c.blocksJoined(parent, "\n\n")
}

func (c *conv) blocksJoined(parent *xhtml.Node, sep string) string {
	var parts []string
	var run []*xhtml.Node
	flush := func() {
		if len(run) > 0 {
			if s := paragraph(c.inlineNodes(run)); s != "" {
				parts = append(parts, s)
			}
			run = nil
		}
	}
	for ch := parent.FirstChild; ch != nil; ch = ch.NextSibling {
		if c.skip(ch) {
			continue
		}
		if ch.Type == xhtml.ElementNode && isBlockEl(ch) {
			flush()
			if s := c.block(ch); s != "" {
				parts = append(parts, s)
			}
			continue
		}
		run = append(run, ch)
	}
	flush()
	return strings.Join(parts, sep)
}

func (c *conv) block(n *xhtml.Node) string {
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := int(n.DataAtom.String()[1] - '0')
		if s := oneLine(c.inline(n)); s != "" {
			return strings.Repeat("#", level) + " " + s
		}
		return ""
	case atom.P:
		return paragraph(c.inline(n))
	case atom.Ul, atom.Ol:
		return c.list(n)
	case atom.Blockquote:
		inner := c.blocks(n)
		if inner == "" {
			return ""
		}
		lines := strings.Split(inner, "\n")
		for i, l := range lines {
			if l == "" {
				lines[i] = ">"
			} else {
				lines[i] = "> " + l
			}
		}
		return strings.Join(lines, "\n")
	case atom.Pre:
		return c.pre(n)
	case atom.Hr:
		return "---"
	case atom.Table:
		return c.table(n)
	case atom.Dl:
		return c.definitions(n)
	case atom.Details:
		return c.details(n)
	case atom.Summary, atom.Caption, atom.Figcaption:
		if s := oneLine(c.inline(n)); s != "" {
			return "*" + strings.Trim(s, "*") + "*"
		}
		return ""
	case atom.Li, atom.Dt, atom.Dd:
		return c.blocks(n) // a stray list item: just its content
	}
	return c.blocks(n) // div, section, article and the other wrappers
}

func (c *conv) list(n *xhtml.Node) string {
	ordered := n.DataAtom == atom.Ol
	number := 1
	if s := attr(n, "start"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			number = v
		}
	}
	var items []string
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != xhtml.ElementNode || li.DataAtom != atom.Li || c.skip(li) {
			continue
		}
		body := c.blocksJoined(li, "\n")
		if strings.TrimSpace(body) == "" {
			continue
		}
		marker := "- "
		if ordered {
			marker = strconv.Itoa(number) + ". "
			number++
		}
		lines := strings.Split(body, "\n")
		pad := strings.Repeat(" ", len(marker))
		for i := range lines {
			if i == 0 {
				lines[i] = marker + lines[i]
			} else if lines[i] != "" {
				lines[i] = pad + lines[i]
			}
		}
		items = append(items, strings.Join(lines, "\n"))
	}
	return strings.Join(items, "\n")
}

var langClass = regexp.MustCompile(`(?:^|\s)(?:language|lang)-([\w+#.-]+)`)

func (c *conv) pre(n *xhtml.Node) string {
	lang := ""
	if m := langClass.FindStringSubmatch(attr(n, "class")); m != nil {
		lang = m[1]
	}
	if code := firstElement(n, atom.Code); code != nil {
		if m := langClass.FindStringSubmatch(attr(code, "class")); m != nil {
			lang = m[1]
		}
	}
	code := strings.Trim(rawText(n), "\r\n")
	if strings.TrimSpace(code) == "" {
		return ""
	}
	fence := "```"
	for strings.Contains(code, fence) {
		fence += "`"
	}
	return fence + lang + "\n" + code + "\n" + fence
}

// rawText is the text of n with its whitespace as it is, and a line break for each <br>.
func rawText(n *xhtml.Node) string {
	var sb strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		switch {
		case n.Type == xhtml.TextNode:
			sb.WriteString(n.Data)
		case n.Type == xhtml.ElementNode && n.DataAtom == atom.Br:
			sb.WriteByte('\n')
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return sb.String()
}

func firstElement(n *xhtml.Node, a atom.Atom) *xhtml.Node {
	if els := elements(n, a); len(els) > 0 {
		return els[0]
	}
	return nil
}

func (c *conv) definitions(n *xhtml.Node) string {
	var parts []string
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type != xhtml.ElementNode || c.skip(ch) {
			continue
		}
		switch ch.DataAtom {
		case atom.Dt:
			if s := oneLine(c.inline(ch)); s != "" {
				parts = append(parts, "**"+strings.Trim(s, "*")+"**")
			}
		case atom.Dd:
			if s := c.blocksJoined(ch, "\n"); s != "" {
				parts = append(parts, "- "+strings.ReplaceAll(s, "\n", "\n  "))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func (c *conv) details(n *xhtml.Node) string {
	var parts []string
	if sum := firstElement(n, atom.Summary); sum != nil {
		if s := oneLine(c.inline(sum)); s != "" {
			parts = append(parts, "**"+strings.Trim(s, "*")+"**")
		}
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == xhtml.ElementNode && ch.DataAtom == atom.Summary {
			continue
		}
		if c.skip(ch) {
			continue
		}
		if ch.Type == xhtml.ElementNode && isBlockEl(ch) {
			if s := c.block(ch); s != "" {
				parts = append(parts, s)
			}
		} else if s := paragraph(c.inlineNodes([]*xhtml.Node{ch})); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n")
}

// table makes a markdown table, or, for a table that only lays out a page, the blocks of its cells.
func (c *conv) table(n *xhtml.Node) string {
	type cell struct {
		node   *xhtml.Node
		header bool
	}
	var rows [][]cell
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != xhtml.ElementNode || c.skip(ch) {
				continue
			}
			switch ch.DataAtom {
			case atom.Thead, atom.Tbody, atom.Tfoot:
				walk(ch)
			case atom.Tr:
				var row []cell
				for td := ch.FirstChild; td != nil; td = td.NextSibling {
					if td.Type == xhtml.ElementNode && (td.DataAtom == atom.Td || td.DataAtom == atom.Th) && !c.skip(td) {
						row = append(row, cell{td, td.DataAtom == atom.Th})
					}
				}
				if len(row) > 0 {
					rows = append(rows, row)
				}
			}
		}
	}
	walk(n)
	if len(rows) == 0 {
		return ""
	}
	layout := false
	columns := 0
	headers := 0
	for _, r := range rows {
		if len(r) > columns {
			columns = len(r)
		}
		for _, cl := range r {
			if cl.header {
				headers++
			}
			if hasBlockContent(cl.node) {
				layout = true
			}
		}
	}
	if columns < 2 || len(elements(n, atom.Table)) > 1 {
		layout = true
	}
	if layout && headers == 0 || layout && len(elements(n, atom.Table)) > 1 {
		var parts []string
		for _, r := range rows {
			for _, cl := range r {
				if s := c.blocks(cl.node); s != "" {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "\n\n")
	}
	line := func(r []cell) string {
		cells := make([]string, columns)
		for i := range cells {
			if i < len(r) {
				s := oneLine(c.inlineNodes(childNodes(r[i].node)))
				cells[i] = s // escapeInline has already escaped any |
			}
		}
		return "| " + strings.Join(cells, " | ") + " |"
	}
	var out []string
	out = append(out, line(rows[0]))
	seps := make([]string, columns)
	for i := range seps {
		seps[i] = "---"
	}
	out = append(out, "| "+strings.Join(seps, " | ")+" |")
	for _, r := range rows[1:] {
		out = append(out, line(r))
	}
	return strings.Join(out, "\n")
}

func childNodes(n *xhtml.Node) []*xhtml.Node {
	var out []*xhtml.Node
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		out = append(out, ch)
	}
	return out
}

// hasBlockContent is true when a table cell holds more than a line of text: paragraphs, lists or other tables.
func hasBlockContent(n *xhtml.Node) bool {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == xhtml.ElementNode {
			switch ch.DataAtom {
			case atom.P, atom.Div, atom.Ul, atom.Ol, atom.Table, atom.Pre, atom.Blockquote, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Dl, atom.Hr:
				return true
			}
		}
		if hasBlockContent(ch) {
			return true
		}
	}
	return false
}

// --- inline ---

func (c *conv) inline(n *xhtml.Node) string { return c.inlineNodes(childNodes(n)) }

func (c *conv) inlineNodes(nodes []*xhtml.Node) string {
	var sb strings.Builder
	add := func(s string) {
		if s == "" {
			return
		}
		cur := sb.String()
		if strings.HasPrefix(s, " ") && (cur == "" || strings.HasSuffix(cur, " ") || strings.HasSuffix(cur, "\n")) {
			s = strings.TrimLeft(s, " ")
		}
		sb.WriteString(s)
	}
	for _, n := range nodes {
		if c.skip(n) {
			continue
		}
		if n.Type == xhtml.TextNode {
			add(escapeInline(spaces(n.Data)))
			continue
		}
		if n.Type != xhtml.ElementNode {
			continue
		}
		switch n.DataAtom {
		case atom.Br:
			sb.WriteString("  \n")
		case atom.Strong, atom.B:
			add(wrap(c.inline(n), "**"))
		case atom.Em, atom.I, atom.Cite, atom.Dfn:
			add(wrap(c.inline(n), "*"))
		case atom.Del, atom.S, atom.Strike:
			add(wrap(c.inline(n), "~~"))
		case atom.Code, atom.Kbd, atom.Samp, atom.Tt:
			add(codeSpan(rawText(n)))
		case atom.A:
			add(c.link(n))
		case atom.Img:
			add(c.image(n))
		case atom.Sup, atom.Sub:
			if s := strings.TrimSpace(c.inline(n)); s != "" {
				tag := n.DataAtom.String()
				add("<" + tag + ">" + s + "</" + tag + ">")
			}
		case atom.Input:
			if _, checked := hasAttr(n, "checked"); checked {
				add("[x] ")
			} else {
				add("[ ] ")
			}
		case atom.Q:
			add("\"" + c.inline(n) + "\"")
		default:
			s := c.inline(n)
			if isBlockEl(n) { // a block inside an inline run, such as a div in a link
				s = " " + strings.TrimSpace(s) + " "
			}
			add(s)
		}
	}
	return sb.String()
}

func hasAttr(n *xhtml.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// wrap puts a marker around s, keeping its edge spaces outside as markdown needs.
func wrap(s, marker string) string {
	inner := strings.TrimSpace(s)
	if inner == "" {
		return s
	}
	lead, trail := "", ""
	if strings.HasPrefix(s, " ") {
		lead = " "
	}
	if strings.HasSuffix(s, " ") {
		trail = " "
	}
	return lead + marker + inner + marker + trail
}

func codeSpan(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	ticks := "`"
	for strings.Contains(s, ticks) {
		ticks += "`"
	}
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		s = " " + s + " "
	}
	return ticks + s + ticks
}

// resolve makes href absolute and keeps only addresses that are safe to follow.
func (c *conv) resolve(href string, schemes ...string) (string, bool) {
	href = strings.TrimSpace(href)
	if href == "" || href[0] == '#' {
		return "", false
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	abs := ref
	if c.base != nil {
		abs = c.base.ResolveReference(ref)
	}
	for _, s := range schemes {
		if abs.Scheme == s {
			return abs.String(), true
		}
	}
	return "", false
}

// destination writes an address as a markdown link target.
func destination(u string) string {
	if strings.ContainsAny(u, " ()<>") {
		return "<" + strings.NewReplacer("<", "%3C", ">", "%3E", " ", "%20").Replace(u) + ">"
	}
	return u
}

func (c *conv) link(n *xhtml.Node) string {
	label := c.inline(n)
	href, ok := c.resolve(attr(n, "href"), "http", "https", "mailto", "file")
	if !ok {
		return label // an in-page anchor or a script link: only its text
	}
	text := strings.TrimSpace(label)
	if text == "" {
		return ""
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\n", " "), "  ", " ")
	lead, trail := "", ""
	if strings.HasPrefix(label, " ") {
		lead = " "
	}
	if strings.HasSuffix(label, " ") {
		trail = " "
	}
	return lead + "[" + text + "](" + destination(href) + ")" + trail
}

func (c *conv) image(n *xhtml.Node) string {
	alt := oneLine(escapeInline(spaces(attr(n, "alt"))))
	src, ok := c.resolve(attr(n, "src"), "http", "https", "file") // not data: addresses, which would be pages long
	if !ok {
		return alt
	}
	return "![" + alt + "](" + destination(src) + ")"
}

var spaceRun = regexp.MustCompile(`[ \t\r\n\f\v]+`)

// spaces turns every run of whitespace into one space, keeping that there was some at the edges.
func spaces(s string) string { return spaceRun.ReplaceAllString(s, " ") }

var inlineSpecials = strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "`", "\\`", "[", `\[`, "]", `\]`, "<", `\<`, "|", `\|`)

func escapeInline(s string) string { return inlineSpecials.Replace(s) }

var blockStart = regexp.MustCompile(`^(#{1,6}\s|>|[-+*]\s|\d+[.)]\s|={3,}|-{3,}|` + "```" + `)`)

// paragraph trims one run of inline text and keeps its start from reading as a block marker.
func paragraph(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\n")
	if s == "" {
		return ""
	}
	if blockStart.MatchString(s) {
		s = `\` + s
	}
	return s
}

func oneLine(s string) string { return collapse(s) }

var manyBlankLines = regexp.MustCompile(`\n{3,}`)

func tidy(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if strings.HasSuffix(l, "  ") && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
			lines[i] = strings.TrimRight(l, " ") // a line break at the end of a paragraph does nothing
		}
	}
	return strings.TrimSpace(manyBlankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// --- simplified HTML ---

func (c *conv) simplify(sb *strings.Builder, n *xhtml.Node) {
	if c.skip(n) {
		return
	}
	switch n.Type {
	case xhtml.TextNode:
		if s := spaces(n.Data); strings.TrimSpace(s) != "" {
			sb.WriteString(xhtml.EscapeString(s))
		} else if strings.Contains(s, " ") {
			sb.WriteByte(' ')
		}
		return
	case xhtml.ElementNode:
		keep := true
		switch n.DataAtom {
		case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.P, atom.Br, atom.Ul, atom.Ol, atom.Li, atom.A, atom.Img,
			atom.Table, atom.Thead, atom.Tbody, atom.Tr, atom.Th, atom.Td, atom.Pre, atom.Code, atom.Blockquote, atom.Strong, atom.B,
			atom.Em, atom.I, atom.Hr, atom.Dl, atom.Dt, atom.Dd, atom.Del, atom.S, atom.Sup, atom.Sub, atom.Details, atom.Summary:
		default:
			keep = false
		}
		if keep {
			sb.WriteString("<" + n.Data)
			switch n.DataAtom {
			case atom.A:
				if href, ok := c.resolve(attr(n, "href"), "http", "https", "mailto", "file"); ok {
					sb.WriteString(` href="` + xhtml.EscapeString(href) + `"`)
				}
			case atom.Img:
				if src, ok := c.resolve(attr(n, "src"), "http", "https", "file"); ok {
					sb.WriteString(` src="` + xhtml.EscapeString(src) + `"`)
				}
				if alt := attr(n, "alt"); alt != "" {
					sb.WriteString(` alt="` + xhtml.EscapeString(alt) + `"`)
				}
			}
			sb.WriteString(">")
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			c.simplify(sb, ch)
		}
		if keep && n.DataAtom != atom.Br && n.DataAtom != atom.Img && n.DataAtom != atom.Hr {
			sb.WriteString("</" + n.Data + ">")
		}
		if !keep && isBlockEl(n) {
			sb.WriteByte('\n')
		}
		return
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		c.simplify(sb, ch)
	}
}
