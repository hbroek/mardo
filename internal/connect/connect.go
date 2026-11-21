// Package connect signs Mardo in to a site with OAuth, so that nobody has to copy a token: the site publishes where
// to sign in (RFC 8414), Mardo registers itself once (RFC 7591), sends the reader to the site's sign-in page in the
// browser with a PKCE proof (RFC 7636), catches the code the browser comes back with on a local address (RFC 8252),
// and trades it for tokens. Tokens are renewed with the refresh token.
//
// Only the site's own address is used: the sign-in, token and registration addresses must be on the same origin as
// the site the reader is signing in to, so a page cannot point Mardo at some other server.
package connect

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultPort is the port of the local address the browser comes back to. A client is registered for one address,
// so the same port is used every time when it is free.
const DefaultPort = 8765

// Timeout is how long a sign-in waits for the reader.
const Timeout = 10 * time.Minute

// Metadata is what a site says about its sign-in.
type Metadata struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	RevocationEndpoint    string   `json:"revocation_endpoint"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
	GrantTypes            []string `json:"grant_types_supported"`
	TokenAuthMethods      []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported       []string `json:"scopes_supported"`
}

// What Mardo tells a site about itself when it registers, for the page where the reader is asked to approve it
// (RFC 7591: client_name, client_uri, logo_uri, policy_uri).
const (
	ClientName = "Mardo"
	ClientURI  = "https://mardo.henryvandenbroek.com"
	LogoURI    = "https://mardo.henryvandenbroek.com/icon.png"
	PolicyURI  = "https://mardo.henryvandenbroek.com/privacy.html"
)

// registrationVersion changes when what Mardo tells a site about itself does. A site cannot be asked to update a
// registration, so a client kept from an older version is registered again.
const registrationVersion = 2

// Client is what a site gave Mardo when it registered.
type Client struct {
	ID       string `json:"id"`
	Secret   string `json:"secret,omitempty"`
	Redirect string `json:"redirect"` // the local address this client was registered for
	Version  int    `json:"v,omitempty"`
}

// Tokens is what a site gives for a sign-in.
type Tokens struct {
	Access    string
	Refresh   string
	Type      string
	ExpiresAt time.Time // zero when the site does not say
	Scope     string
}

// wire is the answer of a token endpoint.
type wire struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

// ErrUnreachable means the site could not be asked (no network, a timeout), as against a site that answered and has no
// sign-in. The first is worth asking again soon, the second is not.
var ErrUnreachable = errors.New("could not reach the site")

// ErrInvalidGrant means the site no longer accepts the refresh token (revoked or expired): the reader must sign in again.
var ErrInvalidGrant = errors.New("the sign-in has ended: sign in to the site again")

// NoRedirects makes a client that does not follow redirects, which a token endpoint must never be sent through.
func NoRedirects(hc *http.Client) *http.Client {
	c := *hc
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

func sameOrigin(a, b string) bool {
	x, err1 := url.Parse(a)
	y, err2 := url.Parse(b)
	return err1 == nil && err2 == nil && x.Scheme == y.Scheme && strings.EqualFold(x.Host, y.Host)
}

func get(ctx context.Context, hc *http.Client, address string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<18))
	return resp.StatusCode, body, nil
}

// Discover reads the sign-in metadata of the site at origin ("https://example.com"). It is an error when the site
// has none, lacks what Mardo needs (the code flow with S256), or names addresses on another origin.
func Discover(ctx context.Context, hc *http.Client, origin string) (Metadata, error) {
	origin = strings.TrimRight(origin, "/")
	hc = NoRedirects(hc)
	var md Metadata
	var unreached error
	answered := false
	for _, path := range []string{"/.well-known/oauth-authorization-server", "/.well-known/openid-configuration"} {
		status, body, err := get(ctx, hc, origin+path)
		if err != nil {
			unreached = err
			continue
		}
		answered = true
		if status != http.StatusOK {
			continue
		}
		md = Metadata{}
		if json.Unmarshal(body, &md) != nil || md.AuthorizationEndpoint == "" || md.TokenEndpoint == "" {
			continue
		}
		if !contains(md.CodeChallengeMethods, "S256") {
			return Metadata{}, errors.New("the site does not support the S256 proof Mardo needs")
		}
		if len(md.GrantTypes) > 0 && !contains(md.GrantTypes, "authorization_code") {
			return Metadata{}, errors.New("the site does not offer the sign-in Mardo uses")
		}
		for _, a := range []string{md.AuthorizationEndpoint, md.TokenEndpoint, md.RegistrationEndpoint, md.RevocationEndpoint} {
			if a != "" && !sameOrigin(a, origin) {
				return Metadata{}, fmt.Errorf("the site's sign-in is on another address (%s); Mardo only signs in at the site itself", a)
			}
		}
		return md, nil
	}
	if !answered && unreached != nil {
		return Metadata{}, fmt.Errorf("%w: %v", ErrUnreachable, unreached)
	}
	return Metadata{}, errors.New("this site does not offer a sign-in that Mardo can use")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Register asks the site for a client for redirect. A site that wants no registration needs none: give a Client.
func Register(ctx context.Context, hc *http.Client, md Metadata, redirect, scope string) (Client, error) {
	if md.RegistrationEndpoint == "" {
		return Client{}, errors.New("the site does not let apps register themselves")
	}
	method := "client_secret_post"
	if len(md.TokenAuthMethods) > 0 && !contains(md.TokenAuthMethods, method) && contains(md.TokenAuthMethods, "none") {
		method = "none"
	}
	body, _ := json.Marshal(map[string]any{
		"client_name": ClientName, "client_uri": ClientURI, "logo_uri": LogoURI, "policy_uri": PolicyURI,
		"redirect_uris": []string{redirect}, "grant_types": []string{"authorization_code", "refresh_token"},
		"response_types": []string{"code"}, "token_endpoint_auth_method": method, "scope": scope,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, md.RegistrationEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return Client{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := NoRedirects(hc).Do(req)
	if err != nil {
		return Client{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var out struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		RedirectURIs []string `json:"redirect_uris"`
		Error        string   `json:"error"`
		Description  string   `json:"error_description"`
	}
	_ = json.Unmarshal(data, &out)
	if (resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK) || out.ClientID == "" {
		return Client{}, fmt.Errorf("the site refused to register Mardo (%d %s)", resp.StatusCode, strings.TrimSpace(out.Error+" "+out.Description))
	}
	if len(out.RedirectURIs) > 0 && !contains(out.RedirectURIs, redirect) {
		return Client{}, errors.New("the site did not keep the address Mardo comes back to")
	}
	return Client{ID: out.ClientID, Secret: out.ClientSecret, Redirect: redirect, Version: registrationVersion}, nil
}

func tokenRequest(ctx context.Context, hc *http.Client, endpoint string, form url.Values) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := NoRedirects(hc).Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer resp.Body.Close()
	var w wire
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&w)
	if resp.StatusCode != http.StatusOK || w.AccessToken == "" {
		if w.Error == "invalid_grant" {
			return Tokens{}, ErrInvalidGrant
		}
		return Tokens{}, fmt.Errorf("the site did not give a token (%d %s)", resp.StatusCode, strings.TrimSpace(w.Error+" "+w.Description))
	}
	t := Tokens{Access: w.AccessToken, Refresh: w.RefreshToken, Type: w.TokenType, Scope: w.Scope}
	if w.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(w.ExpiresIn) * time.Second)
	}
	return t, nil
}

// Refresh trades a refresh token for new tokens. The site may replace the refresh token: the new one is in the answer,
// and the old one must not be used again. When it gives none, the old one is kept.
func Refresh(ctx context.Context, hc *http.Client, tokenEndpoint string, c Client, refresh string) (Tokens, error) {
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {c.ID}}
	if c.Secret != "" {
		form.Set("client_secret", c.Secret)
	}
	t, err := tokenRequest(ctx, hc, tokenEndpoint, form)
	if err != nil {
		return Tokens{}, err
	}
	if t.Refresh == "" {
		t.Refresh = refresh
	}
	return t, nil
}

// Revoke tells the site to forget a token. It is best effort.
func Revoke(ctx context.Context, hc *http.Client, revocationEndpoint string, c Client, token string) error {
	form := url.Values{"token": {token}, "client_id": {c.ID}}
	if c.Secret != "" {
		form.Set("client_secret", c.Secret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := NoRedirects(hc).Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("the site answered %d", resp.StatusCode)
	}
	return nil
}

// Session is a sign-in under way: the reader has been sent to the site, and Mardo waits for the browser to come back.
type Session struct {
	AuthURL string // the sign-in page, to open in the browser

	hc       *http.Client
	md       Metadata
	client   Client
	verifier string
	state    string
	redirect string
	listener net.Listener
	server   *http.Server
	got      chan callback
	once     sync.Once
}

type callback struct{ code, state, err string }

// Client is the client this sign-in uses, to be kept for the next one.
func (s *Session) Client() Client { return s.client }

// Metadata is what the site said about its sign-in.
func (s *Session) Metadata() Metadata { return s.md }

// Begin starts a sign-in at the site at origin: it reads the site's metadata, registers Mardo if need be (a Client kept
// from an earlier sign-in is used when it is for the address that is free now), listens on a local address and builds
// the sign-in page's address. The caller opens AuthURL in the browser, then calls Wait.
func Begin(ctx context.Context, hc *http.Client, origin, scope string, kept *Client, port int) (*Session, error) {
	md, err := Discover(ctx, hc, origin)
	if err != nil {
		return nil, err
	}
	if scope == "" { // the reader sees what is asked for on the site's page, and can refuse
		scope = strings.Join(md.ScopesSupported, " ")
	}
	if port == 0 {
		port = DefaultPort
	}
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil { // another program has the port: any free one, with a client of its own
		if l, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return nil, fmt.Errorf("cannot listen for the browser to come back: %w", err)
		}
	}
	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", l.Addr().(*net.TCPAddr).Port)
	s := &Session{hc: hc, md: md, redirect: redirect, listener: l, got: make(chan callback, 1)}
	if kept != nil && kept.ID != "" && kept.Redirect == redirect && kept.Version == registrationVersion {
		s.client = *kept
	} else if s.client, err = Register(ctx, hc, md, redirect, scope); err != nil {
		l.Close()
		return nil, err
	}
	s.verifier, s.state = random(32), random(16)
	sum := sha256.Sum256([]byte(s.verifier))
	auth, err := url.Parse(md.AuthorizationEndpoint)
	if err != nil {
		l.Close()
		return nil, err
	}
	q := auth.Query()
	q.Set("response_type", "code")
	q.Set("client_id", s.client.ID)
	q.Set("redirect_uri", redirect)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
	q.Set("code_challenge_method", "S256")
	q.Set("state", s.state)
	if scope != "" {
		q.Set("scope", scope)
	}
	auth.RawQuery = q.Encode()
	s.AuthURL = auth.String()

	host := auth.Hostname()
	s.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		if q.Get("state") != s.state { // not the sign-in Mardo started: something else on this computer, or an old page. It does not end the real one.
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, page, "Not for Mardo", "This does not belong to a sign-in that Mardo started.")
			return
		}
		if q.Get("error") != "" || q.Get("code") == "" {
			fmt.Fprintf(w, page, "Not signed in", "Mardo was not given access to "+html.EscapeString(host)+". You can close this window.")
		} else {
			fmt.Fprintf(w, page, "Back in Mardo", "Mardo has the answer of "+html.EscapeString(host)+". You can close this window and go back to Mardo.")
		}
		select {
		case s.got <- callback{code: q.Get("code"), state: q.Get("state"), err: q.Get("error")}:
		default:
		}
	})}
	go s.server.Serve(l)
	return s, nil
}

const page = `<!doctype html><meta charset="utf-8"><title>Mardo</title><body style="font:16px system-ui;max-width:32em;margin:15vh auto;padding:0 1em"><h1>%s</h1><p>%s</p>`

// Wait waits until the browser comes back (or ctx ends, or Timeout passes), checks that it is this sign-in that
// came back, and trades the code for tokens.
func (s *Session) Wait(ctx context.Context) (Tokens, error) {
	defer s.Close()
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	select {
	case cb := <-s.got:
		if cb.state != s.state {
			return Tokens{}, errors.New("an answer came back that does not belong to this sign-in; it was ignored")
		}
		if cb.err != "" {
			if cb.err == "access_denied" {
				return Tokens{}, errors.New("the sign-in was not allowed")
			}
			return Tokens{}, fmt.Errorf("the site refused the sign-in (%s)", cb.err)
		}
		form := url.Values{"grant_type": {"authorization_code"}, "code": {cb.code}, "redirect_uri": {s.redirect}, "client_id": {s.client.ID}, "code_verifier": {s.verifier}}
		if s.client.Secret != "" {
			form.Set("client_secret", s.client.Secret)
		}
		return tokenRequest(ctx, s.hc, s.md.TokenEndpoint, form)
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Tokens{}, errors.New("the sign-in took too long; try again")
		}
		return Tokens{}, errors.New("the sign-in was cancelled")
	}
}

// Close stops listening. It is safe to call more than once.
func (s *Session) Close() {
	s.once.Do(func() {
		s.server.Close()
		s.listener.Close()
	})
}

func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
