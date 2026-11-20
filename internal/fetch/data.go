package fetch

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// IsData reports whether input is a data: address (RFC 2397), a page that is in the address itself.
func IsData(input string) bool {
	s := strings.TrimSpace(input)
	return len(s) > 5 && strings.EqualFold(s[:5], "data:") && strings.Contains(s, ",")
}

// DataURL is the data address of markdown text: the form a page is shared in when it has no server.
func DataURL(text string) string {
	return "data:text/markdown;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte(text))
}

// getData answers a data: address: the page is the text after the comma, in base64 or percent-encoded.
// Without a type it is text/plain, as RFC 2397 has it, and text/plain is shown as markdown.
func (f *Fetcher) getData(u *url.URL) (res *Result, err error) {
	start := time.Now()
	var size int64
	defer func() { f.Log.logData(size, start, err) }()

	s := u.Opaque
	if u.RawQuery != "" {
		s += "?" + u.RawQuery
	}
	meta, data, ok := strings.Cut(s, ",")
	if !ok {
		return nil, errors.New("a data address needs a comma before the data")
	}
	ct, isBase64 := strings.CutSuffix(strings.TrimSpace(meta), ";base64")
	if ct == "" || strings.HasPrefix(ct, ";") {
		ct = "text/plain" + ct
	}
	var body []byte
	if isBase64 {
		data, err = url.PathUnescape(data) // a "+" or "/" may have been escaped
		if err != nil {
			return nil, err
		}
		data = strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, data)
		data = strings.TrimRight(data, "=")
		for _, enc := range []*base64.Encoding{base64.RawStdEncoding, base64.RawURLEncoding} {
			if body, err = enc.DecodeString(data); err == nil {
				break
			}
		}
		if err != nil {
			return nil, errors.New("the data is not valid base64")
		}
	} else {
		text, err := url.PathUnescape(data)
		if err != nil {
			return nil, err
		}
		body = []byte(text)
	}
	if len(body) > MaxBody {
		return nil, errors.New("page larger than the limit")
	}
	size = int64(len(body))
	return &Result{URL: u.String(), Status: http.StatusOK, ContentType: ct, Body: body, Header: http.Header{}}, nil
}
