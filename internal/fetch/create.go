package fetch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// CanAdd says whether the server of a response lets a new document be added at its address with POST (Allow:
// POST on, say, the home page of a blog). Only http and https pages; whether this reader may is known when saving.
func CanAdd(res *Result) bool { return allowsOnWeb(res, http.MethodPost) }

// CanDelete says whether the server of a response lets the document be removed with DELETE.
func CanDelete(res *Result) bool { return allowsOnWeb(res, http.MethodDelete) }

func allowsOnWeb(res *Result, method string) bool {
	if res == nil || res.Status >= 400 || res.Header == nil {
		return false
	}
	u, err := url.Parse(res.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return allows(res.Header, method)
}

// writeClient is the client for a change (PUT, POST, DELETE): it does not follow a redirect. A 307 or 308 would send
// the same change, with its text, to the address the server names, which can be another site; the reader approved
// this server only. The answer is read as it is, and a redirect is an error (see redirected).
func (f *Fetcher) writeClient() *http.Client {
	c := *f.client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

// redirected is the error for a change the server answered with a redirect: nothing was changed, and Mardo does not
// send it on. It is nil for any other answer.
func redirected(resp *http.Response, what string) error {
	if resp.StatusCode < 300 || resp.StatusCode > 399 {
		return nil
	}
	where := resp.Header.Get("Location")
	if where == "" {
		where = "another address"
	}
	return fmt.Errorf("the server redirected %s to %s, which is not followed for a change", what, where)
}

// serverSaid is the server's own words about why it refused: the message of a JSON body, else its text, cut short.
func serverSaid(body []byte) string {
	var j map[string]any
	if json.Unmarshal(body, &j) == nil {
		for _, k := range []string{"message", "detail", "error"} {
			if v, ok := j[k].(string); ok && v != "" {
				return v
			}
		}
	}
	t := strings.TrimSpace(string(body))
	if len(t) > 300 {
		t = t[:300] + "…"
	}
	return t
}

// refusal turns an answer that is not a success into an error that says what to do about it.
func refusal(what string, status int, body []byte) error {
	said := serverSaid(body)
	if said != "" {
		said = ": " + said
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("the server refused to %s (%d): you may need to sign in again, or the sign-in lacks the permission to write%s", what, status, said)
	case http.StatusBadRequest, http.StatusConflict, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return fmt.Errorf("the server did not accept it (%d)%s", status, said)
	case http.StatusNotFound, http.StatusGone:
		return fmt.Errorf("the server says it is not there (%d)%s", status, said)
	}
	return fmt.Errorf("the server answered %d %s%s", status, http.StatusText(status), said)
}

// Create adds a document by sending body (markdown) to rawURL with POST, with the saved credentials, and returns the
// address of the new document from the Location header, resolved against rawURL. It must be on the same server.
func (f *Fetcher) Create(ctx context.Context, rawURL string, body []byte) (string, error) {
	u, err := Normalize(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("only http and https pages can be added to")
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/markdown; charset=utf-8")
	req.Header.Set("User-Agent", userAgent)
	if f.auth != nil {
		f.auth.Apply(req)
	}
	resp, err := f.writeClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err := redirected(resp, "the new page"); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", refusal("add the page", resp.StatusCode, answer)
	}
	where := resp.Header.Get("Location")
	if where == "" { // the body may say it: {"url": "…"}
		var j struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(answer, &j) == nil {
			where = j.URL
		}
	}
	if where == "" {
		return "", errors.New("the server added the page but did not say where it is")
	}
	target, err := u.Parse(where)
	if err != nil || target.Host != u.Host || target.Scheme != u.Scheme {
		return "", errors.New("the server added the page but named an address on another site, which is not followed")
	}
	return target.String(), nil
}

// Delete removes the document at rawURL with DELETE, with the saved credentials. etag, when it is a strong tag, is
// sent as If-Match so a page that changed since it was loaded is not removed.
func (f *Fetcher) Delete(ctx context.Context, rawURL, etag string) error {
	u, err := Normalize(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("only http and https pages can be deleted")
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	if etag != "" && !strings.HasPrefix(etag, "W/") {
		req.Header.Set("If-Match", etag)
	}
	if f.auth != nil {
		f.auth.Apply(req)
	}
	resp, err := f.writeClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	answer, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err := redirected(resp, "the delete"); err != nil {
		return err
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusPreconditionFailed:
		return errors.New("the page changed after you opened it, so it was not deleted; reload to see the new version")
	}
	return refusal("delete the page", resp.StatusCode, answer)
}
