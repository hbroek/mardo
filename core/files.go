package core

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mardo/internal/fetch"
)

// FileEntry is one row of the file browser: a folder, or a markdown or text file.
type FileEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	URL    string `json:"url"` // the file address of a file; empty for a folder
	Folder bool   `json:"folder"`
}

// FolderList is what ListFolder returns: the folder that was listed and what it holds.
type FolderList struct {
	Path    string      `json:"path"`
	Home    string      `json:"home"` // the user's home folder
	Entries []FileEntry `json:"entries"`
}

// browsable reports whether the file browser shows a file with this name.
func browsable(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".txt":
		return true
	}
	return false
}

// within reports whether path is root or lies inside it.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ListFolder lists the folders and the .md, .markdown and .txt files in dir, folders first.
// An empty dir means the user's home folder, which is also as far up as it goes. Names that
// start with a dot are left out.
func (a *Core) ListFolder(dir string) (*FolderList, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	if dir == "" {
		dir = home
	}
	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) || !within(home, dir) {
		return nil, errors.New("only the home folder can be browsed")
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	list := &FolderList{Path: dir, Home: home, Entries: []FileEntry{}}
	for _, it := range items {
		name := it.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		p := filepath.Join(dir, name)
		isDir := it.IsDir()
		if it.Type()&os.ModeSymlink != 0 { // a link counts as what it points to
			if st, err := os.Stat(p); err == nil {
				isDir = st.IsDir()
			}
		}
		switch {
		case isDir:
			list.Entries = append(list.Entries, FileEntry{Name: name, Path: p, Folder: true})
		case browsable(name):
			list.Entries = append(list.Entries, FileEntry{Name: name, Path: p, URL: fetch.FileURL(p)})
		}
	}
	sort.SliceStable(list.Entries, func(i, j int) bool {
		a, b := list.Entries[i], list.Entries[j]
		if a.Folder != b.Folder {
			return a.Folder
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return list, nil
}

// RenamePath gives the file or folder at path a new name, in the same folder, and returns its new
// path. Only things inside the home folder can be renamed, and nothing that already exists is
// overwritten. A file keeps its kind: a name with no extension gets the old one, and a name that
// would make the file disappear from the file browser is refused.
func (a *Core) RenamePath(path, name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) || path == home || !within(home, path) {
		return "", errors.New("only things inside the home folder can be renamed")
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`) || strings.ContainsRune(name, 0) {
		return "", errors.New("that is not a usable name")
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		if filepath.Ext(name) == "" {
			name += filepath.Ext(path)
		}
		if !browsable(name) {
			return "", errors.New("a file here must end in .md, .markdown or .txt")
		}
	}
	dest := filepath.Join(filepath.Dir(path), name)
	if dest == path {
		return path, nil
	}
	if _, err := os.Lstat(dest); err == nil && !strings.EqualFold(dest, path) {
		return "", errors.New("there is already something with that name")
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(path, dest); err != nil {
		return "", err
	}
	return dest, nil
}
