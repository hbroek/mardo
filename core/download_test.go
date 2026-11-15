package core

import "testing"

func TestDownloadName(t *testing.T) {
	for _, c := range []struct{ url, disp, ct, want string }{
		{"https://x.test/a/report.pdf", "", "application/pdf", "report.pdf"},
		{"https://x.test/a/report.pdf", `attachment; filename="q3.pdf"`, "", "q3.pdf"},
		{"https://x.test/a", `attachment; filename="../../etc/passwd"`, "", "passwd"},
		{"https://x.test/", "", "text/html", "x.test"},
		{"https://x.test/img", "", "image/png", "img.png"},
	} {
		if got := downloadName(c.url, c.disp, c.ct); got != c.want {
			t.Errorf("downloadName(%q, %q, %q) = %q, want %q", c.url, c.disp, c.ct, got, c.want)
		}
	}
}
