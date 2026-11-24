package core

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"mardo/internal/fetch"
	"mardo/internal/page"
	"mardo/internal/render"
)

// NewDocumentResult is what saving a new document did: the page now shown, or that the file dialog was canceled.
type NewDocumentResult struct {
	Canceled bool      `json:"canceled"`
	Page     page.Page `json:"page"`
}

// SaveNewDocument writes a document that so far exists only in the window to a file the reader
// chooses, then opens that file in the tab. Canceling the dialog is not an error: the document stays a draft.
func (a *Core) SaveNewDocument(tab int, text string) (NewDocumentResult, error) {
	if len(text) > fetch.MaxBody {
		return NewDocumentResult{}, fmt.Errorf("page larger than %d MB", fetch.MaxBody>>20)
	}
	start, _ := os.UserHomeDir()
	if docs := filepath.Join(start, "Documents"); start != "" {
		if info, err := os.Stat(docs); err == nil && info.IsDir() {
			start = docs
		}
	}
	path, err := a.host.PickSave("Save new page", start, "untitled.md", []Filter{{Name: "Markdown (*.md)", Pattern: "*.md"}})
	if err != nil || path == "" {
		return NewDocumentResult{Canceled: err == nil}, err
	}
	path, err = writeNewDocument(path, text)
	if err != nil {
		return NewDocumentResult{}, err
	}
	return NewDocumentResult{Page: a.browser.Navigate(a.ctx, tab, fetch.FileURL(path))}, nil
}

// writeNewDocument writes text to path, which gets .md when it has no markdown or text extension.
func writeNewDocument(path, text string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".txt":
	default:
		path += ".md"
	}
	return path, os.WriteFile(path, []byte(text), 0o644)
}

// SaveSource saves edited markdown back to the server the page came from, then loads the
// page again so the tab shows (and remembers the version of) what is stored now. editURL and
// etag are what the page was loaded with; the server only gets the text if it is still that version.
func (a *Core) SaveSource(tab int, pageURL, editURL, etag, text string) (page.Page, error) {
	pu, err1 := url.Parse(pageURL)
	eu, err2 := url.Parse(editURL)
	if err1 == nil && pu.Scheme == "data" {
		// There is nothing to save to: the edited text is its own new address, which the tab now shows.
		if len(text) > fetch.MaxBody {
			return page.Page{}, fmt.Errorf("page larger than %d MB", fetch.MaxBody>>20)
		}
		a.pageMu.Lock()
		delete(a.pageCache, pageURL)
		a.pageMu.Unlock()
		return a.browser.Navigate(a.ctx, tab, fetch.DataURL(text)), nil
	}
	if err1 == nil && err2 == nil && pu.Scheme == "file" {
		if eu.String() != pu.String() {
			return page.Page{}, errors.New("this page can only be saved where it was opened from")
		}
		if len(text) > fetch.MaxBody {
			return page.Page{}, fmt.Errorf("page larger than %d MB", fetch.MaxBody>>20)
		}
		if err := fetch.PutFile(editURL, []byte(text), etag); err != nil {
			return page.Page{}, err
		}
		a.pageMu.Lock()
		delete(a.pageCache, pageURL)
		a.pageMu.Unlock()
		if renamed, ok := a.renameNewAction(fetch.FilePath(pu), text); ok { // an action made from a template, now named
			return a.browser.Navigate(a.ctx, tab, renamed), nil
		}
		return a.browser.Reload(a.ctx, tab), nil
	}
	if err := errors.Join(err1, err2); err != nil || pu.Host == "" || pu.Scheme != eu.Scheme || pu.Host != eu.Host {
		return page.Page{}, errors.New("this page can only be saved to the server it came from")
	}
	if len(text) > fetch.MaxBody {
		return page.Page{}, fmt.Errorf("page larger than %d MB", fetch.MaxBody>>20)
	}
	err := a.fetcher.Put(a.ctx, editURL, []byte(text), etag)
	var pending *fetch.PendingError
	if err != nil && !errors.As(err, &pending) {
		return page.Page{}, err
	}
	a.pageMu.Lock()
	delete(a.pageCache, pageURL) // the AI must not answer from the old text
	a.pageMu.Unlock()
	fresh := a.browser.Reload(a.ctx, tab)
	if pending != nil { // waiting for review: the page is still the old text, and the reader is told why
		fresh.Submitted, fresh.SubmittedNote = true, pending.Note
	}
	return fresh, nil
}

// AddPage adds a page on the server the tab is on: text (markdown, with its front matter) is sent to pageURL with
// POST, and the tab then shows the new page, at the address the server names. pageURL is the page that said it
// allows adding (Allow: POST), such as a blog's home page.
func (a *Core) AddPage(tab int, pageURL, text string) (page.Page, error) {
	if len(text) > fetch.MaxBody {
		return page.Page{}, fmt.Errorf("page larger than %d MB", fetch.MaxBody>>20)
	}
	where, err := a.fetcher.Create(a.ctx, pageURL, []byte(text))
	if err != nil {
		return page.Page{}, err
	}
	a.pageMu.Lock()
	delete(a.pageCache, pageURL)
	a.pageMu.Unlock()
	return a.browser.Navigate(a.ctx, tab, where), nil
}

// DeletePage removes the page at pageURL from its server, then loads the address again, which the server now answers
// as gone. etag is the version that was loaded.
func (a *Core) DeletePage(tab int, pageURL, etag string) (page.Page, error) {
	if err := a.fetcher.Delete(a.ctx, pageURL, etag); err != nil {
		return page.Page{}, err
	}
	a.pageMu.Lock()
	delete(a.pageCache, pageURL)
	a.pageMu.Unlock()
	return a.browser.Reload(a.ctx, tab), nil
}

// RenderPreview renders markdown the way the page will look, for the preview beside the editor.
// Relative links and images are resolved against pageURL, the address of the page being edited
// (empty or not an address for a page that was never saved).
func (a *Core) RenderPreview(src, pageURL string) (string, error) {
	var base *url.URL
	if u, err := url.Parse(pageURL); err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "file") {
		base = u
	}
	html, _, err := render.Markdown([]byte(src), base)
	return html, err
}
