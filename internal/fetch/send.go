package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MaxSendReply is how much of the answer to a Send is kept, to tell the reader what the server said.
const MaxSendReply = 2048

// Send makes a request that is not a page load: an action of the reader's, such as posting the page's title and
// address to a service. It uses the saved credentials and cookies of the server, like any request of Mardo's. Only
// http and https addresses are used, and a redirect that would turn the request into another one is an error,
// since the data would not have gone where it was meant to. It returns the status and the start of the answer.
func (f *Fetcher) Send(ctx context.Context, method, rawURL, contentType string, body []byte) (int, []byte, error) {
	u, err := Normalize(rawURL)
	if err != nil {
		return 0, nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return 0, nil, fmt.Errorf("only http and https addresses can be called")
	}
	method = strings.ToUpper(method)
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return 0, nil, fmt.Errorf("unsupported method %q", method)
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return 0, nil, err
	}
	if contentType != "" && len(body) > 0 {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*;q=0.5")
	if f.auth != nil {
		f.auth.Apply(req)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	if resp.Request.Method != method {
		return resp.StatusCode, nil, fmt.Errorf("the server redirected the request to %s, which is not followed", resp.Request.URL)
	}
	reply, _ := io.ReadAll(io.LimitReader(resp.Body, MaxSendReply))
	return resp.StatusCode, reply, nil
}
