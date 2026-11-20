package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

// Editable says whether the server of a response lets the document be replaced with PUT,
// and where: the Link rel="edit" address when it is on the same server, else the address
// the page came from. It is "" when the page cannot be edited. Allow only says what the
// resource supports; whether this user may is only known when saving.
func Editable(res *Result) string {
	if res == nil || res.Status >= 400 || res.Header == nil {
		return ""
	}
	base, err := url.Parse(res.URL)
	if err != nil {
		return ""
	}
	if base.Scheme == "file" {
		if writable(FilePath(base)) {
			return res.URL
		}
		return ""
	}
	if base.Scheme == "data" {
		return res.URL // changing it makes a new address, which the reader is shown to copy
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return ""
	}
	if !allows(res.Header, http.MethodPut) {
		return ""
	}
	for _, link := range res.Header.Values("Link") {
		for _, l := range parseLinks(link) {
			if !hasRel(l.rel, "edit") {
				continue
			}
			target, err := base.Parse(l.href)
			if err == nil && target.Scheme == base.Scheme && target.Host == base.Host {
				return target.String()
			}
		}
	}
	return res.URL
}

func allows(h http.Header, method string) bool {
	for _, v := range h.Values("Allow") {
		for _, m := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(m), method) {
				return true
			}
		}
	}
	return false
}

type link struct{ href, rel, typ string }

// parseLinks reads a Link header value: <href>; rel="edit", <href2>; rel=next; type="text/markdown".
func parseLinks(v string) []link {
	var out []link
	for _, part := range splitLinks(v) {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "<") {
			continue
		}
		end := strings.Index(part, ">")
		if end < 0 {
			continue
		}
		l := link{href: part[1:end]}
		for _, param := range splitParams(part[end+1:]) {
			k, val, ok := strings.Cut(strings.TrimSpace(param), "=")
			if !ok {
				continue
			}
			val = strings.Trim(strings.TrimSpace(val), `"`)
			switch strings.ToLower(strings.TrimSpace(k)) {
			case "rel":
				l.rel = val
			case "type":
				l.typ = val
			}
		}
		out = append(out, l)
	}
	return out
}

// splitParams splits on the semicolons between parameters, not those inside quotes (type="text/markdown; charset=utf-8").
func splitParams(v string) []string {
	var parts []string
	quoted, start := false, 0
	for i, r := range v {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ';' && !quoted:
			parts = append(parts, v[start:i])
			start = i + 1
		}
	}
	return append(parts, v[start:])
}

// Alternate is another version of a page that its Link header points to: Link: </a.md>; rel="alternate"; type="text/markdown".
type Alternate struct {
	Type string // media type, lower case, without parameters
	Href string // absolute http or https address
}

// Alternates lists what the Link headers of a response offer as rel="alternate", in the order given.
func Alternates(res *Result) []Alternate {
	if res == nil || res.Header == nil {
		return nil
	}
	base, err := url.Parse(res.URL)
	if err != nil {
		return nil
	}
	var out []Alternate
	for _, v := range res.Header.Values("Link") {
		for _, l := range parseLinks(v) {
			mt, _, err := mime.ParseMediaType(l.typ)
			if err != nil || !hasRel(l.rel, "alternate") {
				continue
			}
			if u, err := base.Parse(strings.TrimSpace(l.href)); err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "data") { // a data: address holds the page itself
				out = append(out, Alternate{Type: mt, Href: u.String()})
			}
		}
	}
	return out
}

// Negotiated reports whether a markdown response is one address answering with markdown because it was asked
// to (and would answer a browser with the web page): the server says it varies by Accept or names the file it
// sent, and the address is not itself a markdown file.
func Negotiated(res *Result) bool {
	if res == nil || res.Header == nil {
		return false
	}
	mt, _, err := mime.ParseMediaType(res.ContentType)
	if err != nil || (mt != "text/markdown" && mt != "text/x-markdown") {
		return false
	}
	if u, err := url.Parse(res.URL); err != nil || isMarkdownExt(u.Path) {
		return false
	}
	if res.Header.Get("Content-Location") != "" {
		return true
	}
	for _, v := range res.Header.Values("Vary") {
		for _, f := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(f), "accept") || strings.TrimSpace(f) == "*" {
				return true
			}
		}
	}
	return false
}

// splitLinks splits on the commas between links, not those inside <...> or quotes.
func splitLinks(v string) []string {
	var parts []string
	depth, quoted, start := false, false, 0
	for i, r := range v {
		switch {
		case r == '"':
			quoted = !quoted
		case quoted:
		case r == '<':
			depth = true
		case r == '>':
			depth = false
		case r == ',' && !depth:
			parts = append(parts, v[start:i])
			start = i + 1
		}
	}
	return append(parts, v[start:])
}

func hasRel(rels, want string) bool {
	for _, r := range strings.Fields(rels) {
		if strings.EqualFold(r, want) {
			return true
		}
	}
	return false
}

// ErrChanged means the document changed on the server after it was loaded.
var ErrChanged = errors.New("the page changed after you opened it (someone else saved it); reload to see the new version")

// ErrPending means the server accepted a save (202) without applying it yet.
var ErrPending = errors.New("the server accepted the change but has not applied it yet, so the page still shows the old text")

// PendingError is a save the server accepted (202) without applying it; Note is what the server said about it, if anything.
type PendingError struct{ Note string }

func (e *PendingError) Error() string {
	if e.Note == "" {
		return ErrPending.Error()
	}
	return ErrPending.Error() + ": " + e.Note
}

func (e *PendingError) Unwrap() error { return ErrPending }

// pendingNote is the server's own explanation, when its 202 answer has one: the message of a JSON body, else its text.
func pendingNote(body []byte) string {
	var j map[string]any
	if json.Unmarshal(body, &j) == nil {
		for _, k := range []string{"message", "detail", "status", "state", "error"} {
			if v, ok := j[k].(string); ok && v != "" {
				return v
			}
		}
	}
	if t := strings.TrimSpace(string(body)); t != "" {
		return t
	}
	return ""
}

// Put replaces the document at rawURL with body, sending the saved credentials. etag is the
// ETag the page was loaded with; the server refuses the save if the document has changed since.
func (f *Fetcher) Put(ctx context.Context, rawURL string, body []byte, etag string) error {
	u, err := Normalize(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https documents can be saved")
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/markdown; charset=utf-8")
	req.Header.Set("User-Agent", userAgent)
	if etag != "" && !strings.HasPrefix(etag, "W/") { // If-Match needs a strong tag
		req.Header.Set("If-Match", etag)
	}
	if f.auth != nil {
		f.auth.Apply(req)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.Request.Method != http.MethodPut { // a redirect turned it into a GET: nothing was saved
		return fmt.Errorf("the server redirected the save to %s, which is not supported", resp.Request.URL)
	}
	switch {
	case resp.StatusCode == http.StatusAccepted:
		// 202: the server took the change but has not applied it (a review queue, a job). Saying "saved"
		// would be wrong, and the page would reload with the old text.
		note, _ := io.ReadAll(io.LimitReader(resp.Body, 600))
		return &PendingError{Note: pendingNote(note)}
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusPreconditionFailed:
		return ErrChanged
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("the server refused the save (%d): saved credentials for this site may be missing or lack write access", resp.StatusCode)
	}
	return fmt.Errorf("the server answered %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
}
