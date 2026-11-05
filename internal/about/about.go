// Package about describes this build of Mardo: its version, when it was built and what it runs on.
package about

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version is the release number. It is the same as MARKETING_VERSION in ios/project.yml and productVersion in
// wails.json, and a test keeps them equal.
var Version = "0.12.0"

// Date is when the build was made (2006-01-02). The Makefile sets it with -ldflags; without it the date of the
// commit is used, and failing that it stays empty.
var Date string

// Info is what a build knows about itself.
type Info struct {
	Version  string
	Date     string // empty if unknown
	Commit   string // short commit hash, empty if unknown
	Platform string // "macOS (arm64)"
	Go       string
}

var systems = map[string]string{"darwin": "macOS", "ios": "iOS", "windows": "Windows", "linux": "Linux"}

// Current reads the build's details.
func Current() Info {
	in := Info{Version: Version, Date: Date, Platform: platform(), Go: runtime.Version()}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if len(s.Value) > 7 {
					in.Commit = s.Value[:7]
				} else {
					in.Commit = s.Value
				}
			case "vcs.time":
				if in.Date == "" && len(s.Value) >= 10 {
					in.Date = s.Value[:10]
				}
			}
		}
	}
	return in
}

func platform() string {
	name := systems[runtime.GOOS]
	if name == "" {
		name = runtime.GOOS
	}
	return fmt.Sprintf("%s (%s)", name, runtime.GOARCH)
}

// Markdown is the About page. licenses is where the notices page is, as a link.
func (in Info) Markdown(licenses string) string {
	var b strings.Builder
	b.WriteString("# About Mardo\n\n")
	b.WriteString("A browser for Markdown: read, write and publish `.md` files, with AI tools for the page and your bookmarks.\n\n")
	b.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Version | %s |\n", in.Version)
	if in.Date != "" {
		fmt.Fprintf(&b, "| Built | %s |\n", in.Date)
	}
	if in.Commit != "" {
		fmt.Fprintf(&b, "| Commit | %s |\n", in.Commit)
	}
	fmt.Fprintf(&b, "| Platform | %s |\n", in.Platform)
	fmt.Fprintf(&b, "| Go | %s |\n\n", strings.TrimPrefix(in.Go, "go"))
	b.WriteString("Copyright © 2026 Henry. Mardo is open source under the MIT License.\n\n")
	fmt.Fprintf(&b, "- [Open source licenses](%s): the libraries Mardo includes and their licenses\n", licenses)
	b.WriteString("- [Source code](https://github.com/hbroek/mardo)\n")
	b.WriteString("- [Privacy policy](https://mardo.henryvandenbroek.com/privacy.html)\n")
	b.WriteString("- [Website](https://mardo.henryvandenbroek.com)\n")
	return b.String()
}
