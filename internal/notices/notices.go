// Package notices holds the third-party license text linked from the About page.
package notices

import (
	_ "embed"
	"strings"
)

// Text is the contents of THIRD_PARTY_NOTICES.md. The copy in this directory is the one that gets built in;
// the one in the repository root is what people see on GitHub, and a test keeps the two identical.
//
//go:embed THIRD_PARTY_NOTICES.md
var Text string

// Reading is Text for reading in the app: the license texts are plain paragraphs that wrap to the screen
// instead of code blocks that run off the right edge of a phone.
func Reading() string {
	var b strings.Builder
	for _, line := range strings.Split(Text, "\n") {
		if strings.TrimSpace(line) == "~~~~" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
