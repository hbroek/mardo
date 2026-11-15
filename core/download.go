package core

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path"
	"strings"
)

// DownloadAs fetches a linked file or image and asks where to save it. It returns the path,
// or "" when the dialog was cancelled.
func (a *Core) DownloadAs(rawURL string) (string, error) {
	res, err := a.fetcher.Download(a.ctx, rawURL)
	if err != nil {
		return "", err
	}
	if res.Status < 200 || res.Status > 299 {
		return "", fmt.Errorf("the server answered %d", res.Status)
	}
	name := downloadName(res.URL, res.Header.Get("Content-Disposition"), res.ContentType)
	dest, err := a.host.PickSave("Save file", "", name, nil)
	if err != nil || dest == "" {
		return "", err
	}
	return dest, os.WriteFile(dest, res.Body, 0o644)
}

// downloadName picks a file name: the one the server names, else the last part of the address.
func downloadName(rawURL, disposition, contentType string) string {
	if _, params, err := mime.ParseMediaType(disposition); err == nil {
		if n := path.Base(strings.ReplaceAll(params["filename"], `\`, "/")); n != "" && n != "." && n != "/" {
			return n
		}
	}
	name := ""
	if u, err := url.Parse(rawURL); err == nil {
		name = path.Base(u.Path)
		if name == "." || name == "/" {
			name = ""
		}
		if name == "" {
			name = u.Hostname()
		}
	}
	if name == "" {
		name = "download"
	}
	if path.Ext(name) == "" {
		if exts, _ := mime.ExtensionsByType(strings.SplitN(contentType, ";", 2)[0]); len(exts) > 0 {
			name += exts[0]
		}
	}
	return name
}

// ImageData fetches an image and returns it as a data: address, for copying it to the clipboard
// (a page cannot read images from other sites itself).
func (a *Core) ImageData(rawURL string) (string, error) {
	res, err := a.fetcher.GetImage(a.ctx, rawURL)
	if err != nil {
		return "", err
	}
	if res.Status < 200 || res.Status > 299 {
		return "", fmt.Errorf("the server answered %d", res.Status)
	}
	ct := strings.SplitN(res.ContentType, ";", 2)[0]
	if !strings.HasPrefix(ct, "image/") {
		return "", errors.New("that is not an image")
	}
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(res.Body), nil
}
