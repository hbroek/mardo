package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mardo/internal/render"
)

// Actions send the page you are reading somewhere: a service's "post this" page, an API, or the clipboard. Each one
// is a markdown file in the actions folder of the config folder: the front matter says what it is, and the text
// that follows is the body to send (or to copy).
//
//	---
//	name: Weft it                    shown in the menu (default: the file name)
//	icon: favicon                    an emoji or a few letters, a picture next to the file, a web address,
//	                                 or favicon for the icon of the site of the url
//	url: https://example.com/compose?page={{url}}&ptitle={{title}}
//	method: GET                      GET (default), POST, PUT, PATCH or DELETE
//	mode: browser                    browser: open url in the default browser (the default for GET)
//	                                 api: call url without showing anything (the default for the other methods)
//	                                 copy: put the body on the clipboard; no url
//	contentType: application/json    of the body (default: JSON when the body starts with { or [, else text)
//	requires: selection              only runs with text selected in the page
//	order: 10                        where the button goes: lowest first, the others after them by name
//	---
//	{ "url": "{{url}}", "title": "{{title}}" }
//
// The placeholders {{url}}, {{title}}, {{description}}, {{image}}, {{keywords}}, {{selection}}, {{body}} (the
// markdown without its front matter), {{fullbody}} (with it), {{host}}, {{date}} and {{datetime}}, and any key of
// the page's own front matter ({{author}}, {{source.url}}, also as {{page.author}}), are filled in for the page; one
// the page does not have is empty. In an address they are percent-encoded, in a JSON body they are escaped to sit
// inside a string.

//go:embed actions/*.md
var builtInActions embed.FS

const (
	maxActionFile  = 64 << 10
	maxActionIcon  = 64 << 10
	maxActionURL   = 8000
	maxActionBody  = 1 << 20
	maxDescription = 200
)

// ActionInfo is an action as the menu shows it.
type ActionInfo struct {
	ID          string `json:"id"` // the file name without .md
	Name        string `json:"name"`
	Icon        string `json:"icon"` // an emoji or short text, or a data: address of a picture; "" for none
	Mode        string `json:"mode"` // browser, api or copy
	Method      string `json:"method"`
	Host        string `json:"host"`        // where an api action sends the page, for the reader to agree to
	Fingerprint string `json:"fingerprint"` // changes when the file does, so that the agreement is asked again
}

// ActionInput is the page an action is run for, as the reader sees it.
type ActionInput struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Source    string `json:"source"`    // the markdown of the page, when it has any
	Selection string `json:"selection"` // the text the reader selected
}

// ActionResult says what the frontend is to do now.
type ActionResult struct {
	Mode    string `json:"mode"` // open: the address is for the default browser; copy: the text is for the clipboard; done: it was sent
	URL     string `json:"url"`
	Text    string `json:"text"`
	Message string `json:"message"`
}

type action struct {
	ActionInfo
	url, body, contentType string
	needsSelection         bool    // requires: selection
	order                  float64 // order: where the button goes; the ones without come after, by name
}

// actionsFolder is where the actions are, "" when the core keeps its files in memory.
func (a *Core) actionsFolder() string {
	if a.dir == "" {
		return ""
	}
	return filepath.Join(a.dir, "actions")
}

// ActionsFolder is the folder that holds the actions ("" when there is none).
func (a *Core) ActionsFolder() string { return a.actionsFolder() }

// installActions puts the examples in the folder the first time there is none. Later the folder is the reader's.
func installActions(folder string) {
	if _, err := os.Stat(folder); err == nil {
		return
	}
	if os.MkdirAll(folder, 0o755) != nil {
		return
	}
	entries, _ := builtInActions.ReadDir("actions")
	for _, e := range entries {
		if data, err := builtInActions.ReadFile("actions/" + e.Name()); err == nil {
			_ = os.WriteFile(filepath.Join(folder, e.Name()), data, 0o644)
		}
	}
}

var actionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,63}$`)

// readActions parses the files of the actions folder, sorted by name. A file that cannot be used is left out.
func (a *Core) readActions() []action {
	folder := a.actionsFolder()
	if folder == "" {
		return nil
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil
	}
	var out []action
	for _, e := range entries {
		name := e.Name()
		id, ok := strings.CutSuffix(name, ".md")
		if !ok || e.IsDir() || !actionID.MatchString(id) || strings.EqualFold(id, "readme") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(folder, name))
		if err != nil || len(data) > maxActionFile {
			continue
		}
		if act, err := a.parseAction(id, data, folder); err == nil {
			out = append(out, act)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].order != out[j].order {
			return out[i].order < out[j].order
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (a *Core) parseAction(id string, data []byte, folder string) (action, error) {
	meta, body := render.FrontMatter(data)
	if meta == nil {
		return action{}, errors.New("no front matter")
	}
	act := action{url: strings.TrimSpace(meta["url"])}
	act.ID = id
	act.Name = strings.TrimSpace(meta["name"])
	if act.Name == "" {
		act.Name = id
	}
	act.Method = strings.ToUpper(strings.TrimSpace(meta["method"]))
	if act.Method == "" {
		act.Method = http.MethodGet
	}
	switch act.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return action{}, fmt.Errorf("unsupported method %q", act.Method)
	}
	act.Mode = strings.ToLower(strings.TrimSpace(meta["mode"]))
	if act.Mode == "" {
		act.Mode = "browser"
		if act.Method != http.MethodGet {
			act.Mode = "api"
		}
	}
	switch act.Mode {
	case "browser":
		if act.Method != http.MethodGet {
			return action{}, errors.New("an action that opens the browser can only GET")
		}
	case "api":
	case "copy":
		act.Method = ""
	default:
		return action{}, fmt.Errorf("unknown mode %q", act.Mode)
	}
	act.body = strings.TrimLeft(string(body), "\r\n")
	if act.Mode == "copy" {
		act.body = strings.TrimRight(act.body, "\r\n")
	}
	if act.Mode != "copy" {
		if !strings.HasPrefix(strings.ToLower(act.url), "https://") && !strings.HasPrefix(strings.ToLower(act.url), "http://") {
			return action{}, errors.New("url must be an http or https address")
		}
		if h, err := url.Parse(strings.NewReplacer("{{", "", "}}", "").Replace(act.url)); err == nil {
			act.Host = h.Hostname()
		}
	}
	act.contentType = strings.TrimSpace(meta["contenttype"])
	if act.contentType == "" {
		act.contentType = "text/plain; charset=utf-8"
		if t := strings.TrimSpace(act.body); strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") {
			act.contentType = "application/json"
		}
	}
	icon := strings.TrimSpace(meta["icon"])
	if strings.EqualFold(icon, "favicon") { // the icon of the site the action goes to
		icon = ""
		if u, err := url.Parse(strings.NewReplacer("{{", "", "}}", "").Replace(act.url)); err == nil && u.Host != "" {
			icon = faviconPrefix + u.Scheme + "://" + u.Host
		}
	}
	act.Icon = a.actionIcon(icon, folder)
	act.needsSelection = strings.Contains(strings.ToLower(meta["requires"]), "selection")
	act.order = math.MaxFloat64
	if n, err := strconv.ParseFloat(strings.TrimSpace(meta["order"]), 64); err == nil {
		act.order = n
	}
	sum := sha256.Sum256(data)
	act.Fingerprint = hex.EncodeToString(sum[:8])
	return act, nil
}

// actionIcon turns the icon of an action file into something the menu shows: text as it is, a picture file next to
// the action as a data address, and a web address as the picture got from it earlier (see icons.go).
func (a *Core) actionIcon(icon, folder string) string {
	switch {
	case icon == "":
		return ""
	case strings.HasPrefix(icon, "http://"), strings.HasPrefix(icon, "https://"), strings.HasPrefix(icon, faviconPrefix):
		return a.webIcon(icon)
	case strings.ContainsAny(icon, "/\\") || !strings.Contains(icon, "."):
		return icon // an emoji or a few letters
	}
	data, err := os.ReadFile(filepath.Join(folder, filepath.Base(icon)))
	if err != nil || len(data) > maxActionIcon {
		return ""
	}
	return imageDataURL(icon, "", data)
}

// imageDataURL is a picture as a data address, for an <img>. What the picture is comes from what it contains, not
// from its name: LinkaGoGo's favicon.ico is an SVG. served is the type a server said, "" for a file.
func imageDataURL(name, served string, data []byte) string {
	ct := http.DetectContentType(data)
	if bytes.Contains(data[:min(len(data), 1024)], []byte("<svg")) {
		ct = "image/svg+xml"
	}
	if !strings.HasPrefix(ct, "image/") {
		if mt, _, err := mime.ParseMediaType(served); err == nil && strings.HasPrefix(mt, "image/") {
			ct = mt
		} else if strings.EqualFold(filepath.Ext(name), ".ico") && len(data) > 4 && data[0] == 0 && data[1] == 0 && data[2] == 1 {
			ct = "image/x-icon"
		} else {
			return ""
		}
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// Actions lists the actions of the actions folder, which is read each time.
func (a *Core) Actions() []ActionInfo {
	var out []ActionInfo
	for _, act := range a.readActions() {
		out = append(out, act.ActionInfo)
	}
	return out
}

// RunAction runs an action for the page in. What it does depends on its mode: it answers with the address for
// the default browser, with the text for the clipboard, or, when it called an api, with what the server said.
func (a *Core) RunAction(id string, in ActionInput) (ActionResult, error) {
	var act *action
	for _, x := range a.readActions() {
		if x.ID == id {
			act = &x
			break
		}
	}
	if act == nil {
		return ActionResult{}, errors.New("that action is gone; check the actions folder")
	}
	if act.needsSelection && strings.TrimSpace(in.Selection) == "" {
		return ActionResult{}, errors.New("select some text in the page first, for " + act.Name)
	}
	vars := actionVars(in, time.Now())
	switch act.Mode {
	case "copy":
		return ActionResult{Mode: "copy", Text: expandAction(act.body, vars, rawEncoding)}, nil
	case "browser":
		u := expandAction(act.url, vars, urlEncoding)
		if len(u) > maxActionURL {
			return ActionResult{}, errors.New("the address would be too long for a browser; use a POST action for this much")
		}
		a.fetcher.Log.Note("OPEN", u, "opened in the default browser by the action "+act.Name)
		return ActionResult{Mode: "open", URL: u}, nil
	}
	u := expandAction(act.url, vars, urlEncoding)
	if len(u) > maxActionURL {
		return ActionResult{}, errors.New("the address would be too long; send the page in the body of a POST")
	}
	enc := rawEncoding
	if strings.Contains(strings.ToLower(act.contentType), "json") {
		enc = jsonEncoding
	}
	body := expandAction(act.body, vars, enc)
	if len(body) > maxActionBody {
		return ActionResult{}, errors.New("the body is too large to send")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	status, reply, err := a.fetcher.Send(ctx, act.Method, u, act.contentType, []byte(body))
	if err != nil {
		return ActionResult{}, err
	}
	a.fetcher.Log.Note(act.Method, u, fmt.Sprintf("%d, sent by the action %s", status, act.Name))
	note := strings.TrimSpace(string(reply))
	if utf8.RuneCountInString(note) > 160 {
		note = string([]rune(note)[:160]) + "…"
	}
	if status >= 400 {
		msg := fmt.Sprintf("%s: %d %s", act.Name, status, http.StatusText(status))
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			msg += ". Save credentials for " + act.Host + " (menu, Site credentials)"
		}
		if note != "" {
			msg += " — " + note
		}
		return ActionResult{}, errors.New(msg)
	}
	msg := act.Name + ": sent"
	if status == http.StatusAccepted {
		msg += " (accepted, not done yet)"
	}
	if note != "" && len(note) < 80 && !strings.HasPrefix(note, "<") {
		msg += " — " + note
	}
	return ActionResult{Mode: "done", Message: msg}, nil
}

// --- placeholders ---

var actionMacro = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.-]+)\s*\}\}`)

func urlEncoding(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }
func rawEncoding(s string) string { return s }
func jsonEncoding(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1]) // what sits between the quotes
}

// expandAction fills the placeholders of tmpl. One that names nothing the page has is empty.
func expandAction(tmpl string, vars map[string]string, enc func(string) string) string {
	return actionMacro.ReplaceAllStringFunc(tmpl, func(m string) string {
		return enc(vars[strings.ToLower(actionMacro.FindStringSubmatch(m)[1])])
	})
}

var (
	mdImage = regexp.MustCompile(`!\[[^\]]*\]\(\s*<?([^)\s>]+)`)
	mdLink  = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
)

// actionVars are the values of the placeholders for a page.
func actionVars(in ActionInput, now time.Time) map[string]string {
	vars := map[string]string{
		"url": in.URL, "title": strings.TrimSpace(in.Title), "selection": strings.TrimSpace(in.Selection),
		"fullbody": in.Source, "date": now.Format("2006-01-02"), "datetime": now.Format(time.RFC3339),
	}
	base, _ := url.Parse(in.URL)
	if base != nil {
		vars["host"] = base.Hostname()
		if vars["title"] == "" {
			vars["title"] = base.Hostname()
		}
	}
	meta, body := render.FrontMatter([]byte(in.Source))
	vars["body"] = string(body)
	for k, v := range meta { // any key of the front matter, as {{key}} or {{page.key}}; the names above come first
		vars["page."+k] = v
		if _, taken := vars[k]; !taken {
			vars[k] = v
		}
	}
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(meta[k]); v != "" {
				return v
			}
		}
		return ""
	}
	vars["description"] = first("description", "summary")
	if vars["description"] == "" {
		vars["description"] = firstParagraph(string(body))
	}
	vars["keywords"] = first("keywords", "tags")
	img := first("image", "rightmarginimage", "pimg")
	if img == "" {
		if m := mdImage.FindStringSubmatch(string(body)); m != nil {
			img = m[1]
		}
	}
	if img != "" && base != nil {
		if u, err := base.Parse(img); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			img = u.String()
		} else {
			img = ""
		}
	}
	vars["image"] = img
	return vars
}

// firstParagraph is the first line of ordinary text of a markdown document, a few words of it as a description.
func firstParagraph(md string) string {
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.ContainsAny(t[:1], "#>-*+|=`<{") || strings.HasPrefix(t, "![") || strings.HasPrefix(t, "---") || strings.HasPrefix(t, "{{") {
			continue
		}
		t = mdLink.ReplaceAllString(t, "$1")
		t = strings.NewReplacer("*", "", "_", "", "`", "").Replace(t)
		if utf8.RuneCountInString(t) > maxDescription {
			t = strings.TrimSpace(string([]rune(t)[:maxDescription])) + "…"
		}
		return t
	}
	return ""
}
