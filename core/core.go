package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"mardo/internal/auth"
	"mardo/internal/bookmarks"
	"mardo/internal/bookmarks/linkagogo"
	_ "mardo/internal/bookmarks/sqlite"
	"mardo/internal/browser"
	"mardo/internal/chats"
	"mardo/internal/cookies"
	"mardo/internal/fetch"
	"mardo/internal/history"
	"mardo/internal/imgproxy"
	"mardo/internal/localfile"
	"mardo/internal/notices"
	"mardo/internal/page"
	"mardo/internal/render"
	"mardo/internal/settings"
)

// Core holds the state shared with the frontend. Its exported methods are what the frontend calls;
// whatever needs the window or the system (dialogs, menus, events) goes through the Host.
type Core struct {
	ctx     context.Context
	host    Host
	browser *browser.Browser
	auth    *auth.Manager
	jar     *cookies.Jar
	fetcher *fetch.Fetcher

	bmMu         sync.RWMutex
	bookmarks    bookmarks.Store // nil when the backend failed to open
	bookmarksErr error
	settings     settings.Settings
	settingsPath string
	licensesPath string // where Licenses writes the notices page; empty uses the temp directory
	lggURL       string // LinkaGoGo API address; empty is the real one (tests set it)

	pageMu    sync.Mutex
	pageCache map[string]pageCacheEntry // page text for AI features, see pageText
	chats     *chats.Store              // saved Ask this page conversations

	openMu sync.Mutex
	opens  []string // files to open, see QueueOpen

	aiMu      sync.Mutex
	aiCancels map[string]context.CancelFunc // running AI requests, by the id the page chose
}

// Config is what a shell gives New.
type Config struct {
	Dir     string       // where settings, history, bookmarks and other files live; empty keeps them in memory
	Secrets auth.Secrets // where passwords and keys are kept
	Host    Host         // the window, or nil when there is none
}

// New starts the core on cfg. On an error with the files they stay in memory.
func New(cfg Config) *Core {
	dir := cfg.Dir
	file := func(name string) string {
		if dir == "" {
			return ""
		}
		return filepath.Join(dir, name)
	}
	keychain := cfg.Secrets
	if keychain == nil {
		keychain = &auth.MemorySecrets{}
	}
	host := cfg.Host
	if host == nil {
		host = noHost{}
	}
	a := &Core{
		ctx:  context.Background(),
		host: host,
		auth: auth.NewManager(keychain, file("credentials.json")),
		jar:  cookies.New(keychain, file("cookies.bin")),
	}
	a.fetcher = fetch.NewFetcher(a.auth, a.jar)
	a.fetcher.Log.OnEntry = func(e fetch.Entry) { host.Emit("net", e) } // each request is pushed to the console as it completes
	a.browser = browser.New(history.Open(file("history.json")), a.fetcher)

	a.licensesPath = file("licenses.md")
	a.settingsPath = file("settings.json")
	a.settings = settings.Load(a.settingsPath, dir)
	a.browser.SetMarkdownVersions(!a.settings.Browsing.NoMarkdownVersion)
	a.bookmarks, a.bookmarksErr = bookmarks.Open(a.settings.Bookmarks.Backend, a.settings.Bookmarks.Config, bookmarks.Deps{Authorize: a.auth.Apply})
	a.browser.SetSearcher(a.searchPage)
	a.browser.SetMeaningSearcher(a.meaningPage)
	a.chats = chats.Open(file("conversations.json"))
	return a
}

// SetContext makes ctx the context that page loads run under, so they end with the application.
func SetContext(a *Core, ctx context.Context) { a.ctx = ctx }

// Each page-level call names the tab it acts on; NewTab hands out the ids.
// NavigateAsIs opens a page without looking for its .md version.
func (a *Core) NavigateAsIs(tab int, url string) page.Page {
	return a.browser.NavigateAsIs(a.ctx, tab, url)
}

// MarkdownVersions says whether web pages that are not markdown are checked for a .md version.
func (a *Core) MarkdownVersions() bool { return a.browser.MarkdownVersions() }

// SetMarkdownVersions turns that check on or off and remembers the choice.
func (a *Core) SetMarkdownVersions(on bool) error {
	a.browser.SetMarkdownVersions(on)
	a.bmMu.Lock()
	a.settings.Browsing.NoMarkdownVersion = !on
	a.bmMu.Unlock()
	a.host.SetChecked(MarkdownVersionsMenu, on)
	if a.settingsPath == "" {
		return nil
	}
	raw := settings.LoadRaw(a.settingsPath) // not the resolved copy, which would pin defaults
	raw.Browsing.NoMarkdownVersion = !on
	return settings.Save(a.settingsPath, raw)
}

func (a *Core) NewTab() int                            { return a.browser.NewTab() }
func (a *Core) CloseTab(tab int)                       { a.browser.CloseTab(tab) }
func (a *Core) Navigate(tab int, url string) page.Page { return a.browser.Navigate(a.ctx, tab, url) }
func (a *Core) Reload(tab int) page.Page               { return a.browser.Reload(a.ctx, tab) }
func (a *Core) Back(tab int) page.Page                 { return a.browser.Back(a.ctx, tab) }
func (a *Core) Forward(tab int) page.Page              { return a.browser.Forward(a.ctx, tab) }

func (a *Core) History() []history.Entry { return a.browser.History() }
func (a *Core) ClearHistory()            { a.browser.ClearHistory() }

// FullHistory opens the whole visit log as a generated markdown page.
func (a *Core) FullHistory(tab int) page.Page { return a.browser.FullHistory(a.ctx, tab) }

// Licenses opens the open source notices (the libraries Mardo includes and their licenses) as a markdown page.
func (a *Core) Licenses(tab int) page.Page {
	path := a.licensesPath
	if path == "" {
		path = filepath.Join(os.TempDir(), "mardo-licenses.md")
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		_ = os.WriteFile(path, []byte(notices.Reading()), 0o644) // if this fails, opening the file shows the error
	}
	return a.browser.Navigate(a.ctx, tab, path)
}

// Credentials lists saved credentials without their secrets.
func (a *Core) Credentials() []auth.Info { return a.auth.List() }

// SaveCredential stores a credential in the OS keychain and returns the
// normalized site origin it applies to.
func (a *Core) SaveCredential(c auth.Credential) (string, error) { return a.auth.Save(c) }

func (a *Core) DeleteCredential(origin string) error { return a.auth.Delete(origin) }

// ImportCookiesTxt saves the cookies for origin found in a cookies.txt export.
func (a *Core) ImportCookiesTxt(origin, text string) (int, error) {
	return a.auth.ImportCookiesTxt(origin, text)
}

// ClearCookies forgets the cookies that sites have set.
func (a *Core) ClearCookies() { a.jar.Clear() }

// NetworkLog returns the recent requests shown in the console.
func (a *Core) NetworkLog() []fetch.Entry { return a.fetcher.Log.Entries() }

func (a *Core) ClearNetworkLog() { a.fetcher.Log.Clear() }

// Bookmark bindings. They report a backend that failed to open as an error
// instead of stopping the browser from starting.

func (a *Core) store() (bookmarks.Store, error) {
	a.bmMu.RLock()
	defer a.bmMu.RUnlock()
	if a.bookmarks == nil {
		return nil, fmt.Errorf("bookmarks unavailable: %w", a.bookmarksErr)
	}
	return a.bookmarks, nil
}

func (a *Core) bookmarkCtx() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}

// BookmarkFolders lists every folder, flat; the frontend nests them by ParentID.
func (a *Core) BookmarkFolders() ([]bookmarks.Folder, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.Folders(a.bookmarkCtx())
}

// BookmarksIn lists the bookmarks directly in a folder ("" is the root).
func (a *Core) BookmarksIn(folderID string) ([]bookmarks.Bookmark, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.Bookmarks(a.bookmarkCtx(), folderID)
}

// BookmarkFor returns the saved bookmark for a URL, or nil when it is not saved.
func (a *Core) BookmarkFor(url string) (*bookmarks.Bookmark, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	b, err := s.FindByURL(a.bookmarkCtx(), url)
	if errors.Is(err, bookmarks.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// SaveBookmark creates or updates a bookmark; a new one for an
// already-saved URL updates the existing bookmark.
func (a *Core) SaveBookmark(b bookmarks.Bookmark) (bookmarks.Bookmark, error) {
	s, err := a.store()
	if err != nil {
		return b, err
	}
	return bookmarks.SaveUnique(a.bookmarkCtx(), s, b)
}

func (a *Core) DeleteBookmark(id string) error {
	s, err := a.store()
	if err != nil {
		return err
	}
	return s.DeleteBookmark(a.bookmarkCtx(), id)
}

// SaveBookmarkFolder creates (empty ID), renames or moves a folder.
func (a *Core) SaveBookmarkFolder(f bookmarks.Folder) (bookmarks.Folder, error) {
	s, err := a.store()
	if err != nil {
		return f, err
	}
	return s.SaveFolder(a.bookmarkCtx(), f)
}

// DeleteBookmarkFolder removes a folder with everything inside it.
func (a *Core) DeleteBookmarkFolder(id string) error {
	s, err := a.store()
	if err != nil {
		return err
	}
	return s.DeleteFolder(a.bookmarkCtx(), id)
}

// SearchBookmarks finds bookmarks containing every word of the query.
func (a *Core) SearchBookmarks(query string) ([]bookmarks.Bookmark, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.Search(a.bookmarkCtx(), strings.Fields(query))
}

// matchingVisits returns the history entries (most recent first) that contain
// every term in their title or address. No terms means no history: listing
// everything is what Full history is for.
func (a *Core) matchingVisits(terms []string) []bookmarks.Visit {
	if len(terms) == 0 {
		return nil
	}
	var out []bookmarks.Visit
	for _, e := range a.browser.History() {
		if bookmarks.Matches(bookmarks.Bookmark{Title: e.Title, URL: e.URL}, terms) {
			out = append(out, bookmarks.Visit{Title: e.Title, URL: e.URL, Time: e.Time})
		}
	}
	return out
}

// searchPage is the address bar's bookmark and history search: a markdown page of results.
func (a *Core) searchPage(ctx context.Context, query string) (string, error) {
	s, err := a.store()
	if err != nil {
		return "", err
	}
	results, err := s.Search(ctx, strings.Fields(query))
	if err != nil {
		return "", err
	}
	folders, err := s.Folders(ctx)
	if err != nil {
		return "", err
	}
	return bookmarks.SearchMarkdown(query, results, a.matchingVisits(strings.Fields(query)), folders), nil
}

// maxImportBytes bounds the bookmark file read on import.
const maxImportBytes = 64 << 20

// PickFile asks for a local file to open and returns its path, or "" if the dialog is canceled.
func (a *Core) PickFile() (string, error) {
	return a.host.PickOpen("Open page", []Filter{
		{Name: "Markdown and text (*.md, *.markdown, *.txt)", Pattern: "*.md;*.markdown;*.txt"},
		{Name: "All files", Pattern: "*"},
	})
}

// ImportBookmarks asks for a bookmark file and adds its folders and
// bookmarks: an HTML export from any browser, or a markdown list (.md,
// .markdown, .txt; see internal/bookmarks/markdown_io.go). Canceling the
// dialog is not an error; the result says so.
func (a *Core) ImportBookmarks() (bookmarks.ImportResult, error) {
	s, err := a.store()
	if err != nil {
		return bookmarks.ImportResult{}, err
	}
	path, err := a.host.PickOpen("Import bookmarks", []Filter{{Name: "Bookmark files (*.html, *.htm, *.md)", Pattern: "*.html;*.htm;*.md;*.markdown;*.txt"}})
	if err != nil || path == "" {
		return bookmarks.ImportResult{Canceled: err == nil}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return bookmarks.ImportResult{}, err
	}
	defer f.Close()
	parse := bookmarks.ParseNetscape
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".txt":
		parse = bookmarks.ParseMarkdown
	}
	root, err := parse(io.LimitReader(f, maxImportBytes))
	if err != nil {
		return bookmarks.ImportResult{}, err
	}
	return bookmarks.Import(a.bookmarkCtx(), s, root)
}

// ExportBookmarks asks where to save and writes every bookmark as an HTML
// bookmark file (format "html") or a markdown list ("markdown"). It returns
// the path, or "" when the dialog was canceled.
func (a *Core) ExportBookmarks(format string) (string, error) {
	s, err := a.store()
	if err != nil {
		return "", err
	}
	name, pattern, export := "bookmarks.html", "*.html", bookmarks.ExportNetscape
	switch format {
	case "html":
	case "markdown":
		name, pattern, export = "bookmarks.md", "*.md", bookmarks.ExportMarkdown
	default:
		return "", fmt.Errorf("unknown export format %q", format)
	}
	path, err := a.host.PickSave("Export bookmarks", "", name, []Filter{{Name: "Bookmark files (" + pattern + ")", Pattern: pattern}})
	if err != nil || path == "" {
		return "", err
	}
	out, err := export(a.bookmarkCtx(), s)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(out), 0o644)
}

// --- bookmark storage settings ---

// BookmarkSettings describes where bookmarks are kept.
type BookmarkSettings struct {
	Backend   string `json:"backend"`   // "sqlite" or "linkagogo"
	Root      string `json:"root"`      // LinkaGoGo folder used as the top level; "" is the whole account
	Connected bool   `json:"connected"` // a LinkaGoGo API key is saved
	Error     string `json:"error"`     // why the current storage is unavailable, if it is
}

func (a *Core) BookmarkSettings() BookmarkSettings {
	a.bmMu.RLock()
	defer a.bmMu.RUnlock()
	out := BookmarkSettings{Backend: a.settings.Bookmarks.Backend, Root: a.settings.Bookmarks.Config["root"]}
	if a.bookmarksErr != nil {
		out.Error = a.bookmarksErr.Error()
	}
	origin := linkagogoOrigin(a.lggURL)
	for _, c := range a.auth.List() {
		if c.Origin == origin && c.Kind == auth.Bearer && a.auth.Readable(origin) {
			out.Connected = true
		}
	}
	return out
}

func linkagogoOrigin(apiURL string) string {
	s, err := linkagogo.New(bookmarks.Config{"url": apiURL}, bookmarks.Deps{})
	if err != nil {
		return ""
	}
	return s.Origin()
}

// linkagogoStore opens a LinkaGoGo store. With apiKey it signs with that key,
// which is used only for this store and not saved; without, with the saved one.
func (a *Core) linkagogoStore(apiKey, root string) (*linkagogo.Store, error) {
	apiKey = strings.TrimSpace(apiKey)
	if strings.ContainsAny(apiKey, " \t\r\n") {
		return nil, errors.New("the API key must not contain spaces or line breaks")
	}
	authorize := a.auth.Apply
	if apiKey != "" {
		authorize = func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+apiKey) }
	}
	cfg := bookmarks.Config{}
	if a.lggURL != "" {
		cfg["url"] = a.lggURL
	}
	if root != "" {
		cfg["root"] = root
	}
	return linkagogo.New(cfg, bookmarks.Deps{Authorize: authorize})
}

// LinkaGoGoFolders lists every folder in the account so one can be chosen as
// the app's top level. A key typed in the dialog is tried without being saved.
func (a *Core) LinkaGoGoFolders(apiKey string) ([]bookmarks.Folder, error) {
	s, err := a.linkagogoStore(apiKey, "")
	if err != nil {
		return nil, err
	}
	return s.RootCandidates(a.bookmarkCtx())
}

// UseLinkaGoGo keeps bookmarks in LinkaGoGo under the folder rootID ("" is the
// whole account), or under a new top-level folder called newFolder. The key
// and folder are tried first; nothing is saved or switched if they fail.
func (a *Core) UseLinkaGoGo(apiKey, rootID, newFolder string) error {
	ctx := a.bookmarkCtx()
	trial, err := a.linkagogoStore(apiKey, "")
	if err != nil {
		return err
	}
	if name := strings.TrimSpace(newFolder); name != "" {
		f, err := trial.SaveFolder(ctx, bookmarks.Folder{Name: name})
		if err != nil {
			return err
		}
		rootID = f.ID
	}
	if rootID != "" {
		if trial, err = a.linkagogoStore(apiKey, rootID); err != nil {
			return err
		}
	}
	if _, err := trial.Folders(ctx); err != nil { // proves the key works and the folder exists
		return err
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		if _, err := a.auth.Save(auth.Credential{Origin: trial.Origin(), Kind: auth.Bearer, Token: key}); err != nil {
			return err
		}
	}

	cfg := bookmarks.Config{}
	if rootID != "" {
		cfg["root"] = rootID
	}
	if a.lggURL != "" {
		cfg["url"] = a.lggURL
	}
	return a.switchBookmarks("linkagogo", cfg)
}

// UseLocalBookmarks goes back to the SQLite file on this computer.
func (a *Core) UseLocalBookmarks() error {
	cfg := bookmarks.Config{}
	a.bmMu.RLock()
	if a.settings.Bookmarks.Backend == "sqlite" {
		cfg = a.settings.Bookmarks.Config // keep a custom file path
	}
	a.bmMu.RUnlock()
	return a.switchBookmarks("sqlite", cfg)
}

// switchBookmarks opens the backend, saves the choice and starts using it.
func (a *Core) switchBookmarks(backend string, cfg bookmarks.Config) error {
	a.bmMu.RLock()
	aiCfg := a.settings.AI
	a.bmMu.RUnlock()
	next := settings.Settings{Bookmarks: settings.Backend{Backend: backend, Config: cfg}, AI: aiCfg, Browsing: a.settings.Browsing}
	dir := ""
	if a.settingsPath != "" {
		dir = filepath.Dir(a.settingsPath)
	}
	resolved := settings.Resolve(next, dir)
	store, err := bookmarks.Open(resolved.Bookmarks.Backend, resolved.Bookmarks.Config, bookmarks.Deps{Authorize: a.auth.Apply})
	if err != nil {
		return err
	}
	if a.settingsPath != "" {
		if err := settings.Save(a.settingsPath, next); err != nil {
			store.Close()
			return err
		}
	}
	a.bmMu.Lock()
	old := a.bookmarks
	a.bookmarks, a.bookmarksErr, a.settings = store, nil, resolved
	a.bmMu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

// BookmarkItem is a bookmark with the folders above it, for lists that span folders.
type BookmarkItem struct {
	Bookmark bookmarks.Bookmark `json:"bookmark"`
	Path     string             `json:"path"` // "Reading / Rust"; empty at the top level
}

// BookmarksUnder lists every bookmark in a folder and its subfolders ("" is all of them).
func (a *Core) BookmarksUnder(folderID string) ([]BookmarkItem, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	ctx := a.bookmarkCtx()
	folders, err := s.Folders(ctx)
	if err != nil {
		return nil, err
	}
	children := map[string][]bookmarks.Folder{}
	for _, f := range folders {
		children[f.ParentID] = append(children[f.ParentID], f)
	}
	bookmarks.SortFolders(children[folderID])
	var out []BookmarkItem
	var walk func(id, path string) error
	walk = func(id, path string) error {
		list, err := s.Bookmarks(ctx, id)
		if err != nil {
			return err
		}
		for _, b := range list {
			out = append(out, BookmarkItem{Bookmark: b, Path: path})
		}
		kids := children[id]
		bookmarks.SortFolders(kids)
		for _, f := range kids {
			sub := f.Name
			if path != "" {
				sub = path + " / " + f.Name
			}
			if err := walk(f.ID, sub); err != nil {
				return err
			}
		}
		return nil
	}
	start := ""
	for _, f := range folders {
		if f.ID == folderID {
			start = f.Name
		}
	}
	if folderID != "" && start == "" {
		return nil, fmt.Errorf("%w: folder %s", bookmarks.ErrNotFound, folderID)
	}
	return out, walk(folderID, start)
}

// AssetHandler answers the webview's requests for images: files on disk for pages
// opened from disk, and web images that need this app's credentials or plain http.
// (A function, not a method, so it is not offered to the frontend as a call.)
func AssetHandler(a *Core) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(imgproxy.Prefix, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		imgproxy.Handler(a.fetcher).ServeHTTP(w, r) // the fetcher exists once New has run
	}))
	mux.Handle("/", localfile.Handler)
	return mux
}

// SavePage asks where to save the page and writes it as markdown: a markdown page
// as it is, a feed as a document of its entries, any other page as its title and
// links. It returns the path, or "" when the dialog was cancelled.
func (a *Core) SavePage(p page.Page) (string, error) {
	if p.Kind == page.KindError || p.URL == "" {
		return "", errors.New("there is no page to save")
	}
	md := p.Markdown()
	if base, err := url.Parse(p.URL); err == nil && p.Kind == page.KindMarkdown {
		if feed, ok := render.FeedMarkdown([]byte(p.Source), base); ok { // the source of a feed is XML
			md = feed
		}
	}
	path, err := a.host.PickSave("Save page", "", p.FileName(), []Filter{{Name: "Markdown (*.md)", Pattern: "*.md"}})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(md), 0o644)
}
