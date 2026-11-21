// Package auth stores per-site credentials in the OS keychain and attaches
// them to outgoing requests for exactly the origin they were saved for.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Kind string

const (
	Cookie Kind = "cookie" // raw Cookie header value
	Bearer Kind = "bearer"
	Basic  Kind = "basic"
)

// Credential is what the user saves for one origin. It never goes back to the
// frontend; the frontend only sees Info.
type Credential struct {
	Origin    string `json:"origin"`
	Kind      Kind   `json:"kind"`
	Cookie    string `json:"cookie,omitempty"`
	Token     string `json:"token,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	AllowHTTP bool   `json:"allowHttp,omitempty"` // the user accepted sending this over plain http

	// OAuth is set when the token came from signing in to the site (see internal/connect): the token is then renewed
	// before it expires. A token the user typed in has none.
	OAuth *OAuthInfo `json:"oauth,omitempty"`
}

// OAuthInfo is what is needed to renew a token got by signing in.
type OAuthInfo struct {
	ClientID      string    `json:"clientId"`
	ClientSecret  string    `json:"clientSecret,omitempty"`
	Redirect      string    `json:"redirect"`
	TokenURL      string    `json:"tokenUrl"`
	RevocationURL string    `json:"revocationUrl,omitempty"`
	RefreshToken  string    `json:"refreshToken,omitempty"`
	ExpiresAt     time.Time `json:"expiresAt,omitempty"` // zero: the site did not say
	Scope         string    `json:"scope,omitempty"`
}

// expiring says whether the access token has ended or is about to.
func (o *OAuthInfo) expiring() bool {
	return o != nil && o.RefreshToken != "" && !o.ExpiresAt.IsZero() && time.Until(o.ExpiresAt) < time.Minute
}

// Info is the non-secret description of a saved credential.
type Info struct {
	Origin    string `json:"origin"`
	Kind      Kind   `json:"kind"`
	Username  string `json:"username,omitempty"`
	AllowHTTP bool   `json:"allowHttp,omitempty"`
	OAuth     bool   `json:"oauth,omitempty"` // signed in to the site, not a token typed in
	Scope     string `json:"scope,omitempty"`
}

// Refresher gets a credential with new tokens for one that is about to end. It must not change c.
type Refresher func(ctx context.Context, c Credential) (Credential, error)

// Manager implements fetch.Authorizer. Secrets live in the keychain; a small
// index of origins (no secrets) lives in a file so they can be listed.
type Manager struct {
	mu        sync.Mutex
	secrets   Secrets
	indexPath string // empty keeps the index in memory
	index     map[string]Info
	cache     map[string]Credential

	refreshMu sync.Mutex // one renewal at a time: a site that replaces refresh tokens would end the sign-in at the second
	refresher Refresher
	failed    map[string]time.Time // when renewing last failed, so that it is not tried at every request
}

func NewManager(secrets Secrets, indexPath string) *Manager {
	m := &Manager{secrets: secrets, indexPath: indexPath, index: map[string]Info{}, cache: map[string]Credential{}, failed: map[string]time.Time{}}
	if data, err := os.ReadFile(indexPath); err == nil {
		var infos []Info
		if json.Unmarshal(data, &infos) == nil {
			for _, i := range infos {
				m.index[i.Origin] = i
			}
		}
	}
	return m
}

// Origin returns scheme://host[:port] with default ports dropped.
func Origin(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host
}

// NormalizeOrigin accepts "example.com", "https://example.com/path", etc.
func NormalizeOrigin(input string) (string, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", errors.New("site is required")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("invalid site %q", input)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	return Origin(u), nil
}

// AllowHTTPEnv names the environment variable listing hosts, comma separated, that may receive
// credentials over plain http, for a home network or a dev box.
const AllowHTTPEnv = "MARDO_ALLOW_HTTP"

// allowedHTTPHost reports whether host is listed in MARDO_ALLOW_HTTP. Hosts match exactly; there are no wildcards.
func allowedHTTPHost(host string) bool {
	host = strings.ToLower(host)
	for _, h := range strings.Split(os.Getenv(AllowHTTPEnv), ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" && h == host {
			return true
		}
	}
	return false
}

// Allowed reports whether credentials may be sent to origin: https, or http
// only for loopback hosts and the hosts in MARDO_ALLOW_HTTP, so secrets never
// cross the network in clear text unless the user said so.
func Allowed(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	return u.Scheme == "http" && (IsLoopbackHost(u.Hostname()) || allowedHTTPHost(u.Hostname()))
}

func isHTTP(origin string) bool { return strings.HasPrefix(origin, "http://") }

// IsLoopbackHost reports whether host is localhost, *.localhost or a loopback IP.
func IsLoopbackHost(host string) bool {
	host = strings.ToLower(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func secretName(origin string) string { return "cred:" + origin }

func hasControl(s string) bool { return strings.ContainsAny(s, "\r\n\x00") }

func (c Credential) validate() error {
	for _, v := range []string{c.Cookie, c.Token, c.Username, c.Password} {
		if hasControl(v) {
			return errors.New("values must not contain line breaks")
		}
	}
	switch c.Kind {
	case Cookie:
		if strings.TrimSpace(c.Cookie) == "" {
			return errors.New("cookie value is required")
		}
	case Bearer:
		if strings.TrimSpace(c.Token) == "" {
			return errors.New("token is required")
		}
	case Basic:
		if c.Username == "" {
			return errors.New("username is required")
		}
		if strings.Contains(c.Username, ":") {
			return errors.New("username must not contain ':'")
		}
	default:
		return fmt.Errorf("unknown credential type %q", c.Kind)
	}
	return nil
}

// Save stores c in the keychain and returns the normalized origin.
func (m *Manager) Save(c Credential) (string, error) {
	origin, err := NormalizeOrigin(c.Origin)
	if err != nil {
		return "", err
	}
	if !Allowed(origin) && !(c.AllowHTTP && isHTTP(origin)) {
		return "", errors.New("credentials are only sent over https (or http on localhost); tick \"Allow plain http\" to send them unencrypted anyway")
	}
	c.Origin = origin
	if err := c.validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.secrets.Set(secretName(origin), string(data)); err != nil {
		return "", fmt.Errorf("cannot write to the keychain: %w", err)
	}
	m.cache[origin] = c
	info := Info{Origin: origin, Kind: c.Kind, Username: c.Username, AllowHTTP: c.AllowHTTP}
	if c.OAuth != nil {
		info.OAuth, info.Scope = true, c.OAuth.Scope
	}
	m.index[origin] = info
	m.saveIndex()
	return origin, nil
}

func (m *Manager) Delete(origin string) error {
	origin, err := NormalizeOrigin(origin)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.secrets.Delete(secretName(origin)); err != nil {
		return err
	}
	delete(m.cache, origin)
	delete(m.index, origin)
	m.saveIndex()
	return nil
}

func clientName(origin string) string { return "oauthclient:" + origin }

// SaveClient keeps what a site gave Mardo when it registered, for the next sign-in there.
func (m *Manager) SaveClient(origin string, data []byte) error {
	return m.secrets.Set(clientName(origin), string(data))
}

// Client is what SaveClient kept for origin.
func (m *Manager) Client(origin string) ([]byte, bool) {
	raw, err := m.secrets.Get(clientName(origin))
	if err != nil || raw == "" {
		return nil, false
	}
	return []byte(raw), true
}

// List returns the saved credentials without their secrets.
func (m *Manager) List() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Info, 0, len(m.index))
	for _, i := range m.index {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Origin < out[b].Origin })
	return out
}

// Readable reports whether the secret for origin is saved and can actually be read,
// unlike List, which only lists what the index remembers.
func (m *Manager) Readable(origin string) bool {
	_, ok := m.lookup(origin)
	return ok
}

func (m *Manager) lookup(origin string) (Credential, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.index[origin]; !ok {
		return Credential{}, false
	}
	if c, ok := m.cache[origin]; ok {
		return c, true
	}
	raw, err := m.secrets.Get(secretName(origin))
	if err != nil {
		return Credential{}, false
	}
	var c Credential
	if json.Unmarshal([]byte(raw), &c) != nil {
		return Credential{}, false
	}
	m.cache[origin] = c
	return c, true
}

// SetRefresher says how a token got by signing in is renewed.
func (m *Manager) SetRefresher(r Refresher) {
	m.refreshMu.Lock()
	m.refresher = r
	m.refreshMu.Unlock()
}

// Get is the saved credential for origin, for what Mardo does itself (revoking a sign-in); the frontend never has it.
func (m *Manager) Get(origin string) (Credential, bool) {
	origin, err := NormalizeOrigin(origin)
	if err != nil {
		return Credential{}, false
	}
	return m.lookup(origin)
}

// fresh renews a token that has ended or is about to, once, whatever the number of requests that notice. When the site
// refuses, the old token is used (the request then fails with the site's answer) and it is not tried again for a minute.
func (m *Manager) fresh(origin string, c Credential) Credential {
	if c.Kind != Bearer || !c.OAuth.expiring() {
		return c
	}
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	if cur, ok := m.lookup(origin); ok { // someone renewed it while this waited
		c = cur
		if !c.OAuth.expiring() {
			return c
		}
	}
	if m.refresher == nil || time.Since(m.failed[origin]) < time.Minute {
		return c
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	next, err := m.refresher(ctx, c)
	if err != nil {
		m.failed[origin] = time.Now()
		return c
	}
	next.Origin = origin
	if _, err := m.Save(next); err != nil {
		m.failed[origin] = time.Now()
		return c
	}
	delete(m.failed, origin)
	return next
}

// Apply attaches the saved credential, if any, for req's exact origin.
func (m *Manager) Apply(req *http.Request) {
	origin := Origin(req.URL)
	c, ok := m.lookup(origin)
	if !ok || !(Allowed(origin) || (c.AllowHTTP && isHTTP(origin))) {
		return
	}
	c = m.fresh(origin, c)
	switch c.Kind {
	case Cookie:
		req.Header.Set("Cookie", c.Cookie) // the cookie jar appends its own
	case Bearer:
		req.Header.Set("Authorization", "Bearer "+c.Token)
	case Basic:
		req.SetBasicAuth(c.Username, c.Password)
	}
}

func (m *Manager) saveIndex() {
	if m.indexPath == "" {
		return
	}
	infos := make([]Info, 0, len(m.index))
	for _, i := range m.index {
		infos = append(infos, i)
	}
	sort.Slice(infos, func(a, b int) bool { return infos[a].Origin < infos[b].Origin })
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil || os.MkdirAll(filepath.Dir(m.indexPath), 0o755) != nil {
		return
	}
	tmp := m.indexPath + ".tmp"
	if os.WriteFile(tmp, data, 0o600) != nil {
		return
	}
	if os.Rename(tmp, m.indexPath) != nil {
		os.Remove(tmp)
	}
}

// ImportCookiesTxt turns a Netscape cookies.txt export into a Cookie
// credential for origin, keeping only the cookies that belong to that host.
// It returns how many cookies were imported.
func (m *Manager) ImportCookiesTxt(origin, text string) (int, error) {
	origin, err := NormalizeOrigin(origin)
	if err != nil {
		return 0, err
	}
	u, _ := url.Parse(origin)
	host, secure := u.Hostname(), u.Scheme == "https"

	var pairs []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimPrefix(line, "#HttpOnly_")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		domain := strings.ToLower(strings.TrimPrefix(f[0], "."))
		if host != domain && !strings.HasSuffix(host, "."+domain) {
			continue
		}
		if strings.EqualFold(f[3], "TRUE") && !secure {
			continue
		}
		if exp, err := strconv.ParseInt(f[4], 10, 64); err == nil && exp != 0 && time.Unix(exp, 0).Before(time.Now()) {
			continue
		}
		name, value := f[5], strings.Join(f[6:], "\t")
		if name == "" || hasControl(name) || hasControl(value) {
			continue
		}
		pairs = append(pairs, name+"="+value)
	}
	if len(pairs) == 0 {
		return 0, fmt.Errorf("no unexpired cookies for %s in that file", host)
	}
	m.mu.Lock()
	allow := m.index[origin].AllowHTTP // keep the choice made when this site was first saved
	m.mu.Unlock()
	_, err = m.Save(Credential{Origin: origin, Kind: Cookie, Cookie: strings.Join(pairs, "; "), AllowHTTP: allow})
	return len(pairs), err
}
