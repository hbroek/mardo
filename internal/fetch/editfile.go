package fetch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// fileTag is a file's version, the way an ETag is a document's: it changes when the file is written.
func fileTag(info os.FileInfo) string {
	return fmt.Sprintf(`"%x-%x"`, info.ModTime().UnixNano(), info.Size())
}

// writable reports whether a regular file can be opened for writing. Opening does not change it.
func writable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// PutFile replaces a local file with body. A non-empty etag (from the load) must still be the
// file's version, else ErrChanged: something else wrote it since. The write goes to a new file
// that then takes the old one's place, so a failure cannot leave half a document; where that is
// not possible (a folder that cannot be written to) the file is written in place.
func PutFile(rawURL string, body []byte, etag string) error {
	u, err := Normalize(rawURL)
	if err != nil || u.Scheme != "file" {
		return fmt.Errorf("not a local file: %s", rawURL)
	}
	path := FilePath(u)
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real // write the file a link points to, not the link
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if etag != "" && !strings.EqualFold(fileTag(info), etag) {
		return ErrChanged
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mardo-*")
	if err != nil {
		return os.WriteFile(path, body, info.Mode().Perm())
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	_, werr := tmp.Write(body)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	if err := os.Chmod(tmp.Name(), info.Mode().Perm()); err != nil {
		return os.WriteFile(path, body, info.Mode().Perm())
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return os.WriteFile(path, body, info.Mode().Perm())
	}
	return nil
}
