package core

// Filter limits a file dialog to some files: Pattern is a list such as "*.md;*.txt".
type Filter struct {
	Name    string
	Pattern string
}

// Host is what the core asks of the shell around it: the desktop window, or an iPhone app.
type Host interface {
	// PickOpen asks for a file to read and returns its path, or "" when canceled.
	PickOpen(title string, filters []Filter) (string, error)
	// PickSave asks where to write a file, starting in dir (when not empty) with the name given, and returns the path, or "" when canceled.
	PickSave(title, dir, name string, filters []Filter) (string, error)
	// Emit tells the frontend that something happened.
	Emit(event string, data any)
	// SetChecked ticks or unticks the menu item with this id, if the shell has one.
	SetChecked(id string, on bool)
}

// MarkdownVersionsMenu is the id of the menu item that turns the .md lookup on or off.
const MarkdownVersionsMenu = "markdown-versions"

// noHost is the Host of a core without a window: there is no dialog to answer.
type noHost struct{}

func (noHost) PickOpen(string, []Filter) (string, error)                 { return "", nil }
func (noHost) PickSave(string, string, string, []Filter) (string, error) { return "", nil }
func (noHost) Emit(string, any)                                          {}
func (noHost) SetChecked(string, bool)                                   {}
