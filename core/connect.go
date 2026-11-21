package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mardo/internal/auth"
	"mardo/internal/connect"
)

// Signing in to a site: the site publishes where (see internal/connect), the reader is sent to its page in the
// browser, and the token Mardo gets is saved as the site's Bearer credential, with what is needed to renew it. The
// frontend calls ConnectOffer to know whether to show a button, ConnectStart to get the page to open, and ConnectFinish,
// which waits until the reader has answered.

// ConnectOffer says whether Mardo can sign in to a site.
type ConnectOffer struct {
	Origin    string   `json:"origin"`
	Known     bool     `json:"known"` // the site has been asked (ConnectStatus asks nobody)
	Available bool     `json:"available"`
	Reason    string   `json:"reason"` // why not, when it is not available
	Scopes    []string `json:"scopes"` // what the site offers, to be asked for
	SignedIn  bool     `json:"signedIn"`
}

// ConnectStarted is a sign-in under way.
type ConnectStarted struct {
	ID  string `json:"id"`
	URL string `json:"url"` // the site's sign-in page, for the default browser
}

type connectState struct {
	session *connect.Session
	origin  string
	cancel  context.CancelFunc
	ctx     context.Context
}

type connectRegistry struct {
	mu       sync.Mutex
	sessions map[string]*connectState
	offers   map[string]cachedOffer
}

// cachedOffer is what a site said about its sign-in, kept in signin-offers.json so that a restart does not ask again.
type cachedOffer struct {
	Available bool      `json:"available"`
	Scopes    []string  `json:"scopes,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	At        time.Time `json:"at"`
	Unreached bool      `json:"unreached,omitempty"` // the site could not be reached, so the answer is not a real one
}

// How long an answer is kept: a sign-in that is offered for a week, none for a day, no answer at all for an hour.
// So a site is asked about at most a few times a week, and only when the reader needs a sign-in there.
const (
	offerTTL         = 7 * 24 * time.Hour
	noOfferTTL       = 24 * time.Hour
	unreachableTTL   = time.Hour
	signInOffersFile = "signin-offers.json"
)

func (c cachedOffer) fresh() bool {
	ttl := noOfferTTL
	switch {
	case c.Available:
		ttl = offerTTL
	case c.Unreached:
		ttl = unreachableTTL
	}
	return time.Since(c.At) < ttl
}

func (a *Core) offersFile() string {
	if a.dir == "" {
		return ""
	}
	return filepath.Join(a.dir, signInOffersFile)
}

// loadOffers reads the kept answers.
func (a *Core) loadOffers() {
	a.connects.mu.Lock()
	defer a.connects.mu.Unlock()
	a.connects.offers = map[string]cachedOffer{}
	if f := a.offersFile(); f != "" {
		if data, err := os.ReadFile(f); err == nil {
			_ = json.Unmarshal(data, &a.connects.offers)
		}
	}
}

// saveOffers writes the answers that are still worth keeping. a.connects.mu is held.
func (a *Core) saveOffers() {
	f := a.offersFile()
	if f == "" {
		return
	}
	keep := map[string]cachedOffer{}
	for k, v := range a.connects.offers {
		if v.fresh() {
			keep[k] = v
		}
	}
	data, err := json.Marshal(keep)
	if err != nil {
		return
	}
	tmp := f + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, f)
	}
}

// oauthHTTP is the client for the sign-in's own requests: no cookies, whatever the reader has saved for the site.
func oauthHTTP() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func (a *Core) signInOrigin(input string) (string, error) {
	origin, err := auth.NormalizeOrigin(input)
	if err != nil {
		return "", err
	}
	if !auth.Allowed(origin) {
		return "", errors.New("Mardo signs in only over https (or to a site on this computer)")
	}
	return origin, nil
}

func (a *Core) signedInTo(origin string) bool {
	c, ok := a.auth.Get(origin)
	return ok && c.OAuth != nil
}

func offerOf(origin string, c cachedOffer, signedIn bool) ConnectOffer {
	return ConnectOffer{Origin: origin, Known: true, Available: c.Available, Reason: c.Reason, Scopes: c.Scopes, SignedIn: signedIn}
}

// ConnectStatus is what Mardo knows about the sign-in at a site without asking anyone: whether the reader is signed
// in there, and what the site said when it was last asked (Known is false when it never was, or the answer is old).
// It is for labels that are drawn on every page.
func (a *Core) ConnectStatus(site string) ConnectOffer {
	origin, err := auth.NormalizeOrigin(site)
	if err != nil {
		return ConnectOffer{Origin: site}
	}
	signedIn := a.signedInTo(origin)
	a.connects.mu.Lock()
	c, ok := a.connects.offers[origin]
	a.connects.mu.Unlock()
	if ok && c.fresh() {
		return offerOf(origin, c, signedIn)
	}
	return ConnectOffer{Origin: origin, SignedIn: signedIn}
}

// ConnectOffer asks the site whether it offers a sign-in Mardo can use, unless that was asked lately: the answer is
// kept (see offerTTL). Call it where a sign-in is needed, not for every page.
func (a *Core) ConnectOffer(site string) ConnectOffer {
	origin, err := a.signInOrigin(site)
	if err != nil {
		return ConnectOffer{Origin: site, Reason: err.Error()}
	}
	signedIn := a.signedInTo(origin)
	a.connects.mu.Lock()
	c, ok := a.connects.offers[origin]
	a.connects.mu.Unlock()
	if ok && c.fresh() {
		return offerOf(origin, c, signedIn)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	c = cachedOffer{At: time.Now()}
	if md, err := connect.Discover(ctx, oauthHTTP(), origin); err != nil {
		c.Reason, c.Unreached = err.Error(), errors.Is(err, connect.ErrUnreachable)
	} else {
		c.Available, c.Scopes = true, md.ScopesSupported
	}
	a.connects.mu.Lock()
	if a.connects.offers == nil {
		a.connects.offers = map[string]cachedOffer{}
	}
	a.connects.offers[origin] = c
	a.saveOffers()
	a.connects.mu.Unlock()
	return offerOf(origin, c, signedIn)
}

// ConnectStart begins a sign-in at the site: scope is what to ask for ("" for all the site offers). It returns the
// page to open in the browser. Only one sign-in per site at a time: starting another ends the first.
func (a *Core) ConnectStart(site, scope string) (ConnectStarted, error) {
	origin, err := a.signInOrigin(site)
	if err != nil {
		return ConnectStarted{}, err
	}
	var kept *connect.Client
	if raw, ok := a.auth.Client(origin); ok {
		var c connect.Client
		if json.Unmarshal(raw, &c) == nil {
			kept = &c
		}
	}
	ctx, cancel := context.WithCancel(a.ctx)
	sess, err := connect.Begin(ctx, oauthHTTP(), origin, strings.TrimSpace(scope), kept, 0)
	if err != nil {
		cancel()
		return ConnectStarted{}, err
	}
	id := randomID()
	a.connects.mu.Lock()
	if a.connects.sessions == nil {
		a.connects.sessions = map[string]*connectState{}
	}
	for old, s := range a.connects.sessions { // one at a time for a site
		if s.origin == origin {
			s.cancel()
			delete(a.connects.sessions, old)
		}
	}
	a.connects.sessions[id] = &connectState{session: sess, origin: origin, cancel: cancel, ctx: ctx}
	a.connects.mu.Unlock()
	return ConnectStarted{ID: id, URL: sess.AuthURL}, nil
}

// ConnectFinish waits for the reader to answer on the site's page, then saves the sign-in. It returns the saved
// credential's description. It fails when the reader refuses, the sign-in is cancelled or it takes too long.
func (a *Core) ConnectFinish(id string) (auth.Info, error) {
	a.connects.mu.Lock()
	st := a.connects.sessions[id]
	a.connects.mu.Unlock()
	if st == nil {
		return auth.Info{}, errors.New("that sign-in is not running")
	}
	defer func() {
		st.cancel()
		a.connects.mu.Lock()
		delete(a.connects.sessions, id)
		a.connects.mu.Unlock()
	}()
	tok, err := st.session.Wait(st.ctx)
	if err != nil {
		// The site may have forgotten the client kept from before (its page then says it does not know the app and
		// nothing comes back here). A registration costs one request, so the next attempt makes a new one.
		_ = a.auth.SaveClient(st.origin, nil)
		return auth.Info{}, err
	}
	md, client := st.session.Metadata(), st.session.Client()
	cred := auth.Credential{Origin: st.origin, Kind: auth.Bearer, Token: tok.Access, OAuth: &auth.OAuthInfo{
		ClientID: client.ID, ClientSecret: client.Secret, Redirect: client.Redirect,
		TokenURL: md.TokenEndpoint, RevocationURL: md.RevocationEndpoint,
		RefreshToken: tok.Refresh, ExpiresAt: tok.ExpiresAt, Scope: tok.Scope,
	}}
	if _, err := a.auth.Save(cred); err != nil {
		return auth.Info{}, err
	}
	if raw, err := json.Marshal(client); err == nil {
		_ = a.auth.SaveClient(st.origin, raw)
	}
	for _, i := range a.auth.List() {
		if i.Origin == st.origin {
			return i, nil
		}
	}
	return auth.Info{Origin: st.origin, Kind: auth.Bearer, OAuth: true}, nil
}

// ConnectCancel ends a sign-in that is waiting for the reader.
func (a *Core) ConnectCancel(id string) {
	a.connects.mu.Lock()
	st := a.connects.sessions[id]
	a.connects.mu.Unlock()
	if st != nil {
		st.cancel()
	}
}

// refreshSignIn renews a token got by signing in (the auth package calls it just before one would be sent).
func (a *Core) refreshSignIn(ctx context.Context, c auth.Credential) (auth.Credential, error) {
	o := c.OAuth
	if o == nil || o.RefreshToken == "" || o.TokenURL == "" {
		return auth.Credential{}, errors.New("nothing to renew")
	}
	t, err := connect.Refresh(ctx, oauthHTTP(), o.TokenURL, connect.Client{ID: o.ClientID, Secret: o.ClientSecret, Redirect: o.Redirect}, o.RefreshToken)
	if err != nil {
		return auth.Credential{}, err
	}
	next := c
	info := *o
	info.RefreshToken, info.ExpiresAt = t.Refresh, t.ExpiresAt // the site may have replaced the refresh token
	if t.Scope != "" {
		info.Scope = t.Scope
	}
	next.Token, next.OAuth = t.Access, &info
	return next, nil
}

// revokeSignIn tells the site to forget a sign-in that is being removed (the auth package calls it). It is best
// effort: the credential goes either way.
func (a *Core) revokeSignIn(ctx context.Context, c auth.Credential) {
	if c.OAuth == nil || c.OAuth.RevocationURL == "" {
		return
	}
	client := connect.Client{ID: c.OAuth.ClientID, Secret: c.OAuth.ClientSecret}
	if c.OAuth.RefreshToken != "" {
		_ = connect.Revoke(ctx, oauthHTTP(), c.OAuth.RevocationURL, client, c.OAuth.RefreshToken)
	}
	_ = connect.Revoke(ctx, oauthHTTP(), c.OAuth.RevocationURL, client, c.Token)
}

func randomID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprint(err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
