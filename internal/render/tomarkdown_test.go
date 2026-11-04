package render

import (
	"net/url"
	"strings"
	"testing"
)

func convert(t *testing.T, html string, whole bool) string {
	t.Helper()
	base, _ := url.Parse("https://example.com/dir/page")
	_, md := ToMarkdown([]byte(html), "text/html; charset=utf-8", base, whole)
	return md
}

func TestToMarkdownMapping(t *testing.T) {
	cases := []struct{ name, html, want string }{
		{"headings", "<h1>One</h1><h2>Two</h2><h6>Six</h6>", "# One\n\n## Two\n\n###### Six"},
		{"paragraphs", "<p>First</p><p>Second   line</p>", "First\n\nSecond line"},
		{"emphasis", "<p>a <strong>bold</strong>, <em>it</em> and <del>gone</del> <b>x</b></p>", "a **bold**, *it* and ~~gone~~ **x**"},
		{"code", "<p>use <code>go test</code> now</p>", "use `go test` now"},
		{"link", `<p><a href="/x y">here</a></p>`, "[here](https://example.com/x%20y)"},
		{"relative link", `<p><a href="other">o</a></p>`, "[o](https://example.com/dir/other)"},
		{"anchor and script links keep only the text", `<p><a href="#top">up</a> <a href="javascript:alert(1)">no</a></p>`, "up no"},
		{"image", `<p><img src="a.png" alt="An image"></p>`, "![An image](https://example.com/dir/a.png)"},
		{"data image has no address", `<p><img src="data:image/png;base64,AAAA" alt="inline"></p>`, "inline"},
		{"lists", "<ul><li>a</li><li>b<ul><li>c</li></ul></li></ul><ol start=\"3\"><li>x</li><li>y</li></ol>", "- a\n- b\n  - c\n\n3. x\n4. y"},
		{"task list", `<ul><li><input type="checkbox" checked> done</li><li><input type="checkbox"> todo</li></ul>`, "- [x] done\n- [ ] todo"},
		{"quote", "<blockquote><p>Said</p><p>more</p></blockquote>", "> Said\n>\n> more"},
		{"rule", "<p>a</p><hr><p>b</p>", "a\n\n---\n\nb"},
		{"pre with language", `<pre><code class="language-go">fmt.Println("x")
if a < b {}
</code></pre>`, "```go\nfmt.Println(\"x\")\nif a < b {}\n```"},
		{"escapes", "<p>2 * 3 = 6_ [x] a|b</p>", `2 \* 3 = 6\_ \[x\] a\|b`},
		{"paragraph that looks like a heading", "<p># not a heading</p>", `\# not a heading`},
		{"line break", "<p>one<br>two</p>", "one  \ntwo"},
		{"definition list", "<dl><dt>Term</dt><dd>Meaning</dd></dl>", "**Term**\n- Meaning"},
		{"details", "<details><summary>More</summary><p>Hidden text</p></details>", "**More**\n\nHidden text"},
		{"super and subscript", "<p>E=mc<sup>2</sup> H<sub>2</sub>O</p>", "E=mc<sup>2</sup> H<sub>2</sub>O"},
		{"script style and hidden parts are left out", `<p>a</p><script>x()</script><style>p{}</style><div hidden>no</div><p aria-hidden="true">no</p><p>b</p>`, "a\n\nb"},
		{"form controls are left out", `<p>a</p><input type="text"><button>Go</button><select><option>x</option></select>`, "a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.TrimSpace(convert(t, "<html><body>"+tc.html+"</body></html>", true)); got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func TestToMarkdownTable(t *testing.T) {
	got := convert(t, `<table><thead><tr><th>Name</th><th>Qty</th></tr></thead><tbody><tr><td>Apple</td><td>3</td></tr><tr><td>A|B</td><td></td></tr></tbody></table>`, true)
	want := "| Name | Qty |\n| --- | --- |\n| Apple | 3 |\n| A\\|B |  |\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// without header cells the first row becomes the header, as markdown tables need one
	got = convert(t, `<table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>`, true)
	if !strings.HasPrefix(got, "| a | b |\n| --- | --- |\n| c | d |") {
		t.Errorf("headerless table: %q", got)
	}
}

func TestToMarkdownLayoutTableIsFlattened(t *testing.T) {
	got := convert(t, `<table><tr><td><p>Left text</p><ul><li>item</li></ul></td><td><p>Right text</p></td></tr></table>`, true)
	if strings.Contains(got, "|") || !strings.Contains(got, "Left text") || !strings.Contains(got, "- item") || !strings.Contains(got, "Right text") {
		t.Errorf("layout table: %q", got)
	}
}

func TestToMarkdownKeepsTheMainContent(t *testing.T) {
	page := `<html><head><title>The Title</title></head><body>
<nav><a href="/">Home</a><a href="/about">About</a></nav>
<header><p>Site banner</p></header>
<main><h1>Real heading</h1><p>The text of the page that matters, long enough to count as the content of this page. ` + strings.Repeat("More words of the article. ", 10) + `</p></main>
<aside>Related links</aside><footer>Copyright</footer></body></html>`
	got := convert(t, page, false)
	if !strings.HasPrefix(got, "# Real heading") || strings.Contains(got, "Home") || strings.Contains(got, "banner") || strings.Contains(got, "Copyright") {
		t.Errorf("main only: %q", got)
	}
	all := convert(t, page, true)
	for _, want := range []string{"Home", "Site banner", "Real heading", "Related links", "Copyright"} {
		if !strings.Contains(all, want) {
			t.Errorf("whole page lacks %q: %q", want, all)
		}
	}
}

func TestToMarkdownWithoutMainDropsNavigation(t *testing.T) {
	page := `<html><body><nav><a href="/">Home</a></nav><div><h2>Section</h2><p>` + strings.Repeat("Words in the body of the page. ", 10) + `</p></div><footer>Copyright</footer></body></html>`
	got := convert(t, page, false)
	if strings.Contains(got, "Home") || strings.Contains(got, "Copyright") || !strings.Contains(got, "## Section") {
		t.Errorf("got %q", got)
	}
}

func TestToMarkdownFallsBackToTheWholePageWhenNothingIsLeft(t *testing.T) {
	got := convert(t, `<html><body><header><h1>All of it is in a header</h1><p>Some words</p></header></body></html>`, false)
	if !strings.Contains(got, "All of it is in a header") {
		t.Errorf("got %q", got)
	}
}

func TestToMarkdownAddsTheTitleWhenThePageHasNoHeading(t *testing.T) {
	_, md := ToMarkdown([]byte(`<html><head><title>My page</title></head><body><p>Hello</p></body></html>`), "text/html", nil, true)
	if md != "# My page\n\nHello\n" {
		t.Errorf("got %q", md)
	}
}

func TestSimplifiedHTMLKeepsStructureAndDropsTheRest(t *testing.T) {
	page := `<html><head><style>x{}</style></head><body><div class="wrap" onclick="evil()"><h1 id="a" style="x">Hi</h1>
<p>Some <a href="/l" target="_blank" onclick="x()">link</a> and <img src="i.png" alt="pic" width="3"></p><script>x()</script><nav>menu</nav></div></body></html>`
	base, _ := url.Parse("https://example.com/")
	_, got := SimplifiedHTML([]byte(page), "text/html", base, false)
	for _, bad := range []string{"onclick", "style", "class", "script", "menu", "target", "width", "id="} {
		if strings.Contains(got, bad) {
			t.Errorf("simplified html still has %q: %s", bad, got)
		}
	}
	for _, want := range []string{"<h1>Hi</h1>", `<a href="https://example.com/l">link</a>`, `<img src="https://example.com/i.png" alt="pic">`} {
		if !strings.Contains(got, want) {
			t.Errorf("simplified html lacks %q: %s", want, got)
		}
	}
}
