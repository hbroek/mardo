package core

import (
	"strings"
	"testing"
)

func TestRenderPreview(t *testing.T) {
	c := &Core{}
	for _, base := range []string{"new:document", "", "https://example.com/dir/a.md"} {
		html, err := c.RenderPreview("# Hi\n\n<script>x()</script>\n\n[l](b.md)", base)
		if err != nil || !strings.Contains(html, "<h1") || strings.Contains(html, "<script") {
			t.Errorf("base %q: %q %v", base, html, err)
		}
	}
	html, _ := c.RenderPreview("[l](b.md)", "https://example.com/dir/a.md")
	if !strings.Contains(html, "https://example.com/dir/b.md") {
		t.Errorf("relative link not resolved: %s", html)
	}
}
