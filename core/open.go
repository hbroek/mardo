package core

import "path/filepath"

// Files the system asked the app to open (a double click in the file manager, an argument on the
// command line) wait here until the page takes them: it may not be up yet when the request comes.

// QueueOpen asks the page to open path. It is a function rather than a method so that the frontend cannot call it.
func QueueOpen(a *Core, path string) {
	a.openMu.Lock()
	a.opens = append(a.opens, path)
	a.openMu.Unlock()
	a.host.Emit("open-requests", nil)
}

// TakeOpenRequests returns the files the system asked to open since the last call.
func (a *Core) TakeOpenRequests() []string {
	a.openMu.Lock()
	defer a.openMu.Unlock()
	out := append([]string{}, a.opens...)
	a.opens = nil
	return out
}

// OpenablePath says whether a command-line argument names a document the app can open, and where it is
// (workDir is where the other instance was started, for relative names).
func OpenablePath(workDir, arg string) (string, bool) {
	switch filepath.Ext(arg) {
	case ".md", ".markdown", ".mdown", ".txt":
	default:
		return "", false
	}
	if !filepath.IsAbs(arg) && workDir != "" {
		arg = filepath.Join(workDir, arg)
	}
	return arg, true
}
