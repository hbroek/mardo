package site

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every page of the website has a markdown twin and says so, so Mardo and other readers can ask for it.
func TestEveryPageHasAMarkdownVersion(t *testing.T) {
	pages, err := filepath.Glob("../../docs/*.html")
	if err != nil || len(pages) == 0 {
		t.Fatalf("no pages found: %v", err)
	}
	for _, p := range pages {
		if strings.HasPrefix(filepath.Base(p), "_") {
			continue // the template for generated pages
		}
		md := strings.TrimSuffix(filepath.Base(p), ".html") + ".md"
		if _, err := os.Stat(filepath.Join(filepath.Dir(p), md)); err != nil {
			t.Errorf("%s has no %s", filepath.Base(p), md)
		}
		html, _ := os.ReadFile(p)
		if !strings.Contains(string(html), `<link rel="alternate" type="text/markdown" href="`+md+`">`) {
			t.Errorf("%s does not link to %s", filepath.Base(p), md)
		}
	}
}

var (
	headingRe = regexp.MustCompile(`(?s)<h([1-3])[^>]*>(.*?)</h[1-3]>`)
	tagRe     = regexp.MustCompile(`<[^>]+>`)
	hrefRe    = regexp.MustCompile(`href="(https?://[^"#]+)"`)
	pageRe    = regexp.MustCompile(`href="([\w-]+\.html)"`) // a link to another page of the site, relative
)

// plain is HTML text as it reads: no tags, entities decoded, spaces collapsed.
func plain(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&middot;", "·", "&hellip;", "…", "&nbsp;", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// The two versions say the same: every heading and every outside link of the HTML page is in the markdown.
// Change one, change the other.
func TestMarkdownSaysWhatTheHTMLSays(t *testing.T) {
	pages, _ := filepath.Glob("../../docs/*.html")
	for _, p := range pages {
		if strings.HasPrefix(filepath.Base(p), "_") {
			continue
		}
		htmlBytes, _ := os.ReadFile(p)
		md, err := os.ReadFile(strings.TrimSuffix(p, ".html") + ".md")
		if err != nil {
			continue // reported by the other test
		}
		html := string(htmlBytes)
		if i := strings.Index(html, "<body"); i >= 0 {
			html = html[i:]
		}
		mdText := string(md)
		mdPlain := strings.NewReplacer("`", "", "*", "").Replace(mdText) // headings compared without their markup
		name := filepath.Base(p)
		for _, m := range headingRe.FindAllStringSubmatch(html, -1) {
			h := plain(m[2])
			if m[1] == "1" && name == "index.html" {
				continue // the home page's title is worded differently in the markdown
			}
			if !strings.Contains(mdPlain, h) {
				t.Errorf("%s: heading %q is missing from the markdown", name, h)
			}
		}
		for _, m := range pageRe.FindAllStringSubmatch(html, -1) {
			if twin := strings.TrimSuffix(m[1], ".html") + ".md"; !strings.Contains(mdText, "("+twin+")") {
				t.Errorf("%s: the link to %s is missing from the markdown, which should link to %s", name, m[1], twin)
			}
		}
		for _, m := range hrefRe.FindAllStringSubmatch(html, -1) {
			link := strings.ReplaceAll(m[1], "&amp;", "&")
			if strings.HasPrefix(link, "https://mardo.henryvandenbroek.com/") {
				link = strings.TrimSuffix(link, ".html") + ".md" // a link to a page of this site is a link to its twin
			}
			if !strings.Contains(mdText, link) {
				t.Errorf("%s: link %s is missing from the markdown", name, m[1])
			}
		}
	}
}

// A link inside a page goes to a heading that exists.
func TestPageAnchorsPointAtHeadings(t *testing.T) {
	anchorRe := regexp.MustCompile(`href="#([^"]+)"`)
	pages, _ := filepath.Glob("../../docs/*.html")
	for _, p := range pages {
		b, _ := os.ReadFile(p)
		html := string(b)
		for _, m := range anchorRe.FindAllStringSubmatch(html, -1) {
			if !strings.Contains(html, `id="`+m[1]+`"`) {
				t.Errorf("%s: the link #%s goes nowhere", filepath.Base(p), m[1])
			}
		}
	}
}
