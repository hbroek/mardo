package core

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mardo/internal/fetch"
	"mardo/internal/page"
)

// Editing the actions inside Mardo, for the phones, where the actions folder is not reachable from a file manager (and
// for anyone who would rather not leave Mardo). EditActions opens a page that lists the actions with a link to each
// file, which opens in the editor like any page, and three links that make a new action from a template. Mardo does
// this with its own features: a file is a page, and the pencil edits it.

// newActionPrefix starts the name of the links that make a new action: _new-browser.md and so on. No such file
// exists; following the link (see newActionLink) makes the action and opens it.
const newActionPrefix = "_new-"

// listPageName is the page EditActions writes. Its name starts with an underscore, so it is not an action.
const listPageName = "_actions.md"

var actionTemplates = map[string]string{
	"browser": `---
# Opens a web page in your default browser, with the page you are reading filled in.
# The placeholders: {{url}} {{title}} {{description}} {{image}} {{keywords}} {{selection}} {{body}} {{host}} {{date}}
# and any key of the page's front matter. They are percent-encoded in the address. See README.md.
name: New action
icon: favicon
order: 100
url: https://example.com/post?link={{url}}&title={{title}}
---
`,
	"api": `---
# Sends the page to a web service without showing anything. Mardo asks you to agree the first time, and uses the
# sign-in you saved for the site (menu, Site credentials). The text after the front matter is the body.
name: New action
icon: favicon
order: 100
method: POST
url: https://example.com/api/posts
---
{
  "link": "{{url}}",
  "title": "{{title}}",
  "note": "{{selection}}"
}
`,
	"copy": `---
# Puts text on the clipboard. The text after the front matter is what is copied.
name: New action
icon: 📋
order: 100
mode: copy
---
[{{title}}]({{url}})
`,
}

// EditActions opens the page that lists the actions in a tab: each links to its file, which opens in the editor.
func (a *Core) EditActions(tab int) (page.Page, error) {
	folder := a.actionsFolder()
	if folder == "" {
		return page.Page{}, fmt.Errorf("there is no actions folder")
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return page.Page{}, err
	}
	if _, err := os.Stat(filepath.Join(folder, "README.md")); err != nil { // the page links to it
		if data, err := builtInActions.ReadFile("actions/README.md"); err == nil {
			_ = os.WriteFile(filepath.Join(folder, "README.md"), data, 0o644)
		}
	}
	var b strings.Builder
	b.WriteString("# Send to actions\n\n")
	b.WriteString("Each action is a markdown file. Open one, tap the pencil to change it, then Save. The Send to toolbar shows the change when you come back to it. This page is made again each time you open it.\n\n")
	b.WriteString("## Your actions\n\n")
	acts := a.readActions()
	if len(acts) == 0 {
		b.WriteString("None yet. Make one below.\n")
	}
	for _, act := range acts {
		what := map[string]string{"browser": "opens " + act.Host + " in your browser", "api": act.Method + " to " + act.Host, "copy": "copies text"}[act.Mode]
		fmt.Fprintf(&b, "- [%s](%s) — %s\n", strings.NewReplacer("[", `\[`, "]", `\]`).Replace(act.Name), url.PathEscape(act.ID+".md"), what)
	}
	b.WriteString("\n## Make a new action\n\n")
	b.WriteString("- [Open a web page in the browser](" + newActionPrefix + "browser.md): a service's post page, a search\n")
	b.WriteString("- [Call a web service](" + newActionPrefix + "api.md): POST the page's title and address, without showing anything\n")
	b.WriteString("- [Copy text](" + newActionPrefix + "copy.md): a link, a quote, a snippet\n\n")
	b.WriteString("[What every field means, with examples](README.md)\n")
	path := filepath.Join(folder, listPageName)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return page.Page{}, err
	}
	return a.browser.Navigate(a.ctx, tab, fetch.FileURL(path)), nil
}

// newActionLink makes the action that a "new action" link stands for, and returns the address of its file. It is
// only for the links EditActions writes: a file address in the actions folder that is named _new-<kind>.md.
func (a *Core) newActionLink(rawURL string) (string, bool) {
	folder := a.actionsFolder()
	u, err := url.Parse(rawURL)
	if err != nil || folder == "" || u.Scheme != "file" {
		return "", false
	}
	path := fetch.FilePath(u)
	if filepath.Clean(filepath.Dir(path)) != filepath.Clean(folder) {
		return "", false
	}
	kind, ok := strings.CutSuffix(strings.TrimPrefix(filepath.Base(path), newActionPrefix), ".md")
	tmpl, known := actionTemplates[kind]
	if !ok || !strings.HasPrefix(filepath.Base(path), newActionPrefix) || !known {
		return "", false
	}
	for n := 1; n < 1000; n++ {
		name := "new-action.md"
		if n > 1 {
			name = fmt.Sprintf("new-action-%d.md", n)
		}
		target := filepath.Join(folder, name)
		f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			continue
		}
		_, werr := f.WriteString(tmpl)
		f.Close()
		if werr != nil {
			return "", false
		}
		return fetch.FileURL(target), true
	}
	return "", false
}

var newActionName = regexp.MustCompile(`^new-action(-\d+)?\.md$`)
var slugChars = regexp.MustCompile(`[^a-z0-9]+`)

// renameNewAction gives an action that was made from a template, and has been given a name of its own, a file name
// that says what it is: "Tweet it" in new-action.md becomes tweet-it.md. Only the files the templates made are
// renamed, and never over another file. It returns the address of the file after the rename.
func (a *Core) renameNewAction(path, text string) (string, bool) {
	folder := a.actionsFolder()
	if folder == "" || filepath.Clean(filepath.Dir(path)) != filepath.Clean(folder) || !newActionName.MatchString(filepath.Base(path)) {
		return "", false
	}
	act, err := a.parseAction("x", []byte(text), folder)
	if err != nil || strings.EqualFold(strings.TrimSpace(act.Name), "new action") || act.Name == "x" {
		return "", false
	}
	slug := strings.Trim(slugChars.ReplaceAllString(strings.ToLower(act.Name), "-"), "-")
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	if slug == "" || slug == "readme" {
		return "", false
	}
	for n := 1; n < 100; n++ {
		name := slug + ".md"
		if n > 1 {
			name = fmt.Sprintf("%s-%d.md", slug, n)
		}
		target := filepath.Join(folder, name)
		if _, err := os.Lstat(target); err == nil {
			continue
		}
		if os.Rename(path, target) != nil {
			return "", false
		}
		return fetch.FileURL(target), true
	}
	return "", false
}
