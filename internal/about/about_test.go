package about

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestVersionMatchesTheOtherPlaces(t *testing.T) {
	yml, err := os.ReadFile("../../ios/project.yml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`MARKETING_VERSION: "([^"]+)"`).FindSubmatch(yml)
	if m == nil || string(m[1]) != Version {
		t.Errorf("ios/project.yml version %q, about.Version %q", m, Version)
	}
	wails, err := os.ReadFile("../../wails.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wails), `"productVersion": "`+Version+`"`) {
		t.Errorf("wails.json needs productVersion %s", Version)
	}
}

func TestMarkdownHasVersionDateAndLicensesLink(t *testing.T) {
	in := Info{Version: "1.2.3", Date: "2026-11-05", Commit: "abc1234", Platform: "macOS (arm64)", Go: "go1.25"}
	md := in.Markdown("licenses.md")
	for _, want := range []string{"| Version | 1.2.3 |", "| Built | 2026-11-05 |", "| Commit | abc1234 |", "[Open source licenses](licenses.md)", "| Go | 1.25 |"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in\n%s", want, md)
		}
	}
	if strings.Contains((Info{Version: "1"}).Markdown("x"), "Built") {
		t.Error("an unknown date should leave the row out")
	}
}
