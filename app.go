package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"mardo/core"
	"mardo/internal/auth"
)

// App is the desktop shell around the core: the window, its menus and its file dialogs.
// The core's exported methods are what Wails offers the frontend as calls.
type App struct {
	*core.Core
	ctx          context.Context
	markdownItem *menu.MenuItem            // Tools > Look for Markdown Versions
	viewItems    map[string]*menu.MenuItem // the View menu's show/hide items (macOS)
}

func NewApp() *App {
	dir, _ := os.UserConfigDir()
	if dir != "" {
		dir = filepath.Join(dir, "mardo")
	}
	// Without a system keyring (bare Linux desktops) secrets go to an encrypted file.
	var keychain auth.Secrets = auth.KeyringStore{}
	if dir != "" {
		keychain = auth.WithFallback{Primary: auth.KeyringStore{}, Fallback: auth.NewFileStore(dir)}
	}
	a := &App{}
	a.Core = core.New(core.Config{Dir: dir, Secrets: keychain, Host: wailsHost{a}})
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	core.SetContext(a.Core, ctx)
	for _, arg := range os.Args[1:] { // a file the system opened the program with
		if path, ok := core.OpenablePath("", arg); ok {
			core.QueueOpen(a.Core, path)
		}
	}
}

// secondInstance opens the files another launch was given and brings the window forward.
func (a *App) secondInstance(data options.SecondInstanceData) {
	for _, arg := range data.Args {
		if path, ok := core.OpenablePath(data.WorkingDirectory, arg); ok {
			core.QueueOpen(a.Core, path)
		}
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.Show(a.ctx)
}

// wailsHost answers the core's requests of the window with the Wails runtime.
type wailsHost struct{ app *App }

func filters(in []core.Filter) []runtime.FileFilter {
	out := make([]runtime.FileFilter, len(in))
	for i, f := range in {
		out[i] = runtime.FileFilter{DisplayName: f.Name, Pattern: f.Pattern}
	}
	return out
}

func (h wailsHost) PickOpen(title string, fs []core.Filter) (string, error) {
	return runtime.OpenFileDialog(h.app.ctx, runtime.OpenDialogOptions{Title: title, Filters: filters(fs)})
}

func (h wailsHost) PickSave(title, dir, name string, fs []core.Filter) (string, error) {
	return runtime.SaveFileDialog(h.app.ctx, runtime.SaveDialogOptions{Title: title, DefaultDirectory: dir, DefaultFilename: name, Filters: filters(fs)})
}

func (h wailsHost) Emit(event string, data any) {
	if h.app.ctx != nil { // before the window is up there is nobody to tell
		runtime.EventsEmit(h.app.ctx, event, data)
	}
}

func (h wailsHost) SetChecked(id string, on bool) {
	if id == core.MarkdownVersionsMenu {
		h.app.checkMenu(h.app.markdownItem, on)
	}
}
