package core

import (
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The page styles. A style is a CSS file in the stylesheets folder of the config folder, in the format of
// Bonbon's stylesheets: tokens on :root, the page on body and .page, and the elements of the rendered
// markdown. The frontend limits each rule to the page (see frontend/src/userstyle.js). The built-in five are
// copied into the folder when they are missing, so they can be edited, and a deleted one comes back at the next start.

//go:embed styles/*.css
var builtInStyles embed.FS

// StyleInfo names one page style.
type StyleInfo struct {
	ID   string `json:"id"`   // the file name without .css
	Name string `json:"name"` // as shown in the menu
}

const (
	maxStyleBytes   = 2 << 20 // all of one style, with what it imports
	maxStyleFont    = 1 << 20 // one font file that is built into the style
	maxStyleImports = 4
)

var styleID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,63}$`)

// stylesFolder is where the styles are, "" when the core keeps its files in memory.
func (a *Core) stylesFolder() string {
	if a.dir == "" {
		return ""
	}
	return filepath.Join(a.dir, "stylesheets")
}

// installStyles puts the built-in styles in folder where there is no file of that name, and leaves the others alone.
func installStyles(folder string) {
	if os.MkdirAll(folder, 0o755) != nil {
		return
	}
	entries, _ := builtInStyles.ReadDir("styles")
	for _, e := range entries {
		target := filepath.Join(folder, e.Name())
		if _, err := os.Stat(target); err == nil {
			continue
		}
		if data, err := builtInStyles.ReadFile("styles/" + e.Name()); err == nil {
			_ = os.WriteFile(target, data, 0o644)
		}
	}
}

// styleFS is the folder of styles, or the built-in ones when there is no folder.
func (a *Core) styleFS() fs.FS {
	if f := a.stylesFolder(); f != "" {
		return os.DirFS(f)
	}
	sub, _ := fs.Sub(builtInStyles, "styles")
	return sub
}

// StylesFolder is the folder that holds the page styles ("" when there is none).
func (a *Core) StylesFolder() string { return a.stylesFolder() }

// StyleSheets lists the page styles: the .css files of the stylesheets folder, which is read each time.
func (a *Core) StyleSheets() []StyleInfo {
	entries, err := fs.ReadDir(a.styleFS(), ".")
	if err != nil {
		return nil
	}
	var out []StyleInfo
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".css")
		if !ok || e.IsDir() || !styleID.MatchString(id) || strings.HasPrefix(id, ".") {
			continue
		}
		name := strings.NewReplacer("-", " ", "_", " ").Replace(id)
		out = append(out, StyleInfo{ID: id, Name: strings.ToUpper(name[:1]) + name[1:]})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

var (
	cssImport = regexp.MustCompile(`@import\s+(?:url\(\s*)?["']?([^"'()\s;]+)["']?\s*\)?\s*;`)
	cssURL    = regexp.MustCompile(`url\(\s*["']?([^"')]*)["']?\s*\)`)
)

// StyleSheet is the CSS of the style id with its imports in place, and the fonts it names, which sit beside it,
// built in as data: addresses. Anything it asks of the network is left out, so a page style cannot make requests.
func (a *Core) StyleSheet(id string) (string, error) {
	if !styleID.MatchString(id) {
		return "", fmt.Errorf("no page style %q", id)
	}
	budget := maxStyleBytes
	css, err := readStyle(a.styleFS(), id+".css", 0, &budget)
	if err != nil {
		return "", err
	}
	return css, nil
}

func readStyle(fsys fs.FS, name string, depth int, budget *int) (string, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	if *budget -= len(data); *budget < 0 {
		return "", fmt.Errorf("the style is larger than %d MB", maxStyleBytes>>20)
	}
	dir := path.Dir(name)
	local := func(ref string) (string, bool) {
		if ref == "" || strings.Contains(ref, ":") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "//") {
			return "", false // an address on the network, a data: address or an absolute path
		}
		p := path.Clean(path.Join(dir, strings.SplitN(ref, "?", 2)[0]))
		if !fs.ValidPath(p) {
			return "", false // out of the folder
		}
		return p, true
	}
	css := cssImport.ReplaceAllStringFunc(string(data), func(m string) string {
		ref := cssImport.FindStringSubmatch(m)[1]
		p, ok := local(ref)
		if !ok || depth >= maxStyleImports || !strings.HasSuffix(p, ".css") {
			return ""
		}
		if inner, err := readStyle(fsys, p, depth+1, budget); err == nil {
			return inner
		}
		return ""
	})
	return cssURL.ReplaceAllStringFunc(css, func(m string) string {
		sub := cssURL.FindStringSubmatch(m)
		ref := sub[1]
		if strings.HasPrefix(ref, "data:") {
			return m
		}
		p, ok := local(ref)
		if !ok {
			return `url("")`
		}
		font, err := fs.ReadFile(fsys, p)
		if err != nil || len(font) > maxStyleFont {
			return `url("")`
		}
		kind := mime.TypeByExtension(strings.ToLower(path.Ext(p)))
		switch strings.ToLower(path.Ext(p)) {
		case ".woff2":
			kind = "font/woff2"
		case ".woff":
			kind = "font/woff"
		case ".ttf":
			kind = "font/ttf"
		case ".otf":
			kind = "font/otf"
		case ".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp":
			// pictures of the style, such as a texture, are built in the same way
		default:
			return `url("")`
		}
		return `url("data:` + kind + `;base64,` + base64.StdEncoding.EncodeToString(font) + `")`
	}), nil
}
