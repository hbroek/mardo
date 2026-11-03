package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"mardo/internal/fetch"
	"mardo/internal/page"
)

// NewDocumentResult is what saving a new document did: the page now shown, or that the file dialog was canceled.
type NewDocumentResult struct {
	Canceled bool      `json:"canceled"`
	Page     page.Page `json:"page"`
}

// SaveNewDocument writes a document that so far exists only in the window to a file the reader
// chooses, then opens that file in the tab. Canceling the dialog is not an error: the document stays a draft.
func (a *App) SaveNewDocument(tab int, text string) (NewDocumentResult, error) {
	if len(text) > fetch.MaxBody {
		return NewDocumentResult{}, fmt.Errorf("page larger than %d MB", fetch.MaxBody>>20)
	}
	start, _ := os.UserHomeDir()
	if docs := filepath.Join(start, "Documents"); start != "" {
		if info, err := os.Stat(docs); err == nil && info.IsDir() {
			start = docs
		}
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "Save new page",
		DefaultDirectory: start,
		DefaultFilename:  "untitled.md",
		Filters:          []runtime.FileFilter{{DisplayName: "Markdown (*.md)", Pattern: "*.md"}},
	})
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
func (a *App) SaveSource(tab int, pageURL, editURL, etag, text string) (page.Page, error) {
	pu, err1 := url.Parse(pageURL)
	eu, err2 := url.Parse(editURL)
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
		return a.browser.Reload(a.ctx, tab), nil
	}
	if err := errors.Join(err1, err2); err != nil || pu.Host == "" || pu.Scheme != eu.Scheme || pu.Host != eu.Host {
		return page.Page{}, errors.New("this page can only be saved to the server it came from")
	}
	if len(text) > fetch.MaxBody {
		return page.Page{}, fmt.Errorf("page larger than %d MB", fetch.MaxBody>>20)
	}
	if err := a.fetcher.Put(a.ctx, editURL, []byte(text), etag); err != nil {
		return page.Page{}, err
	}
	a.pageMu.Lock()
	delete(a.pageCache, pageURL) // the AI must not answer from the old text
	a.pageMu.Unlock()
	return a.browser.Reload(a.ctx, tab), nil
}
