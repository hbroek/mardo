package core

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"mardo/internal/auth"
)

// oauthSite is a site with a sign-in (a loopback address, so it may be used without https) and a private page.
type oauthSite struct {
	*httptest.Server
	mu        sync.Mutex
	codes     map[string]string
	valid     map[string]bool // access tokens that work
	refreshes map[string]bool
	grants    []string // what the token endpoint was asked for
	revoked   []string
	seq       int
	asked     int // how many times the sign-in metadata was fetched
}

func newOAuthSite(t *testing.T) *oauthSite {
	s := &oauthSite{codes: map[string]string{}, valid: map[string]bool{}, refreshes: map[string]bool{}}
	mux := http.NewServeMux()
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.asked++
		s.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{
			"issuer": s.URL, "authorization_endpoint": s.URL + "/oauth/authorize", "token_endpoint": s.URL + "/oauth/token",
			"registration_endpoint": s.URL + "/oauth/register", "revocation_endpoint": s.URL + "/oauth/revoke",
			"code_challenge_methods_supported": []string{"S256"}, "grant_types_supported": []string{"authorization_code", "refresh_token"},
			"scopes_supported": []string{"bookmarks:read", "folders:read"},
		})
	})
	mux.HandleFunc("/oauth/register", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"client_id": "cid", "client_secret": "sec", "redirect_uris": in["redirect_uris"]})
	})
	mux.HandleFunc("/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		code := "code-" + q.Get("state")
		s.mu.Lock()
		s.codes[code] = q.Get("code_challenge")
		s.mu.Unlock()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code="+code+"&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.grants = append(s.grants, r.Form.Get("grant_type"))
		bad := func() { w.WriteHeader(400); io.WriteString(w, `{"error":"invalid_grant"}`) }
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if s.codes[r.Form.Get("code")] != base64.RawURLEncoding.EncodeToString(sum[:]) {
				bad()
				return
			}
		case "refresh_token":
			if !s.refreshes[r.Form.Get("refresh_token")] {
				bad()
				return
			}
			delete(s.refreshes, r.Form.Get("refresh_token"))
		}
		s.seq++
		access, refresh := "at-"+string(rune('a'+s.seq)), "rt-"+string(rune('a'+s.seq))
		s.valid[access], s.refreshes[refresh] = true, true
		json.NewEncoder(w).Encode(map[string]any{"access_token": access, "token_type": "Bearer", "refresh_token": refresh, "expires_in": 3600, "scope": "bookmarks:read folders:read"})
	})
	mux.HandleFunc("/oauth/revoke", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		s.mu.Lock()
		s.revoked = append(s.revoked, r.Form.Get("token"))
		delete(s.valid, r.Form.Get("token"))
		s.mu.Unlock()
	})
	mux.HandleFunc("/app/new", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		ok := s.valid[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		s.mu.Unlock()
		if !ok {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "text/markdown")
		io.WriteString(w, "# New\n\nYour bookmarks.\n")
	})
	return s
}

func TestSigningInToASiteAndUsingTheToken(t *testing.T) {
	site := newOAuthSite(t)
	a := newTestCore(t, t.TempDir())

	offer := a.ConnectOffer(site.URL)
	if !offer.Available || offer.SignedIn || len(offer.Scopes) != 2 {
		t.Fatalf("offer = %+v", offer)
	}
	// Before the sign-in the private page is refused.
	if p := a.Navigate(0, site.URL+"/app/new"); p.Kind != "error" {
		t.Fatalf("the private page showed without a sign-in: %+v", p)
	}

	started, err := a.ConnectStart(site.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(started.URL, site.URL+"/oauth/authorize?") || !strings.Contains(started.URL, "scope=bookmarks%3Aread+folders%3Aread") {
		t.Fatalf("the sign-in address = %s", started.URL)
	}
	go func() { // the reader opens it in the browser and presses Allow
		if resp, err := http.Get(started.URL); err == nil {
			resp.Body.Close()
		}
	}()
	info, err := a.ConnectFinish(started.ID)
	if err != nil || !info.OAuth || info.Kind != auth.Bearer || info.Origin != site.URL || !strings.Contains(info.Scope, "folders:read") {
		t.Fatalf("info = %+v, %v", info, err)
	}
	if !a.ConnectOffer(site.URL).SignedIn {
		t.Error("the offer does not say the reader is signed in")
	}
	// The saved credential is the site's Bearer: the private page now shows, as markdown.
	if p := a.Navigate(0, site.URL+"/app/new"); p.Kind != "markdown" || !strings.Contains(p.HTML, "Your bookmarks") {
		t.Fatalf("the private page after signing in: kind %q error %q", p.Kind, p.Error)
	}
	// The token never goes to the frontend.
	for _, i := range a.Credentials() {
		raw, _ := json.Marshal(i)
		if strings.Contains(string(raw), "at-") || strings.Contains(string(raw), "rt-") || strings.Contains(string(raw), "sec") {
			t.Errorf("a secret is in what the frontend sees: %s", raw)
		}
	}
}

func TestTheTokenIsRenewedWhenItEnds(t *testing.T) {
	site := newOAuthSite(t)
	a := newTestCore(t, t.TempDir())
	started, _ := a.ConnectStart(site.URL, "bookmarks:read")
	go func() {
		resp, err := http.Get(started.URL)
		if err == nil {
			resp.Body.Close()
		}
	}()
	if _, err := a.ConnectFinish(started.ID); err != nil {
		t.Fatal(err)
	}
	// The access token ends (an hour passes).
	c, _ := a.auth.Get(site.URL)
	c.OAuth.ExpiresAt = time.Now().Add(-time.Minute)
	if _, err := a.auth.Save(c); err != nil {
		t.Fatal(err)
	}
	site.mu.Lock()
	delete(site.valid, c.Token) // the site has forgotten it
	site.mu.Unlock()
	if p := a.Navigate(0, site.URL+"/app/new"); p.Kind != "markdown" {
		t.Fatalf("after the token ended: kind %q error %q", p.Kind, p.Error)
	}
	if got := strings.Join(site.grants, ","); got != "authorization_code,refresh_token" {
		t.Errorf("the token endpoint was asked for %s", got)
	}
	if now, _ := a.auth.Get(site.URL); now.Token == c.Token || now.OAuth.RefreshToken == c.OAuth.RefreshToken {
		t.Errorf("the new tokens were not kept")
	}
}

func TestSigningOutRevokesAtTheSite(t *testing.T) {
	site := newOAuthSite(t)
	a := newTestCore(t, t.TempDir())
	started, _ := a.ConnectStart(site.URL, "")
	go func() {
		resp, err := http.Get(started.URL)
		if err == nil {
			resp.Body.Close()
		}
	}()
	if _, err := a.ConnectFinish(started.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteCredential(site.URL); err != nil {
		t.Fatal(err)
	}
	if len(site.revoked) != 2 { // the refresh token and the access token
		t.Errorf("revoked %v", site.revoked)
	}
	if len(a.Credentials()) != 0 {
		t.Errorf("the credential is still there: %+v", a.Credentials())
	}
	// Signing in again uses the client kept from the first time, without registering another.
	again, err := a.ConnectStart(site.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	a.ConnectCancel(again.ID)
}

func TestSignInRefusals(t *testing.T) {
	a := newTestCore(t, t.TempDir())
	if offer := a.ConnectOffer("http://example.com"); offer.Available {
		t.Errorf("a sign-in over plain http was offered: %+v", offer)
	}
	empty := httptest.NewServer(http.NotFoundHandler())
	defer empty.Close()
	if offer := a.ConnectOffer(empty.URL); offer.Available || offer.Reason == "" {
		t.Errorf("a site with no sign-in: %+v", offer)
	}
	if _, err := a.ConnectStart(empty.URL, ""); err == nil {
		t.Error("started a sign-in at a site that has none")
	}
	if _, err := a.ConnectFinish("nothing"); err == nil {
		t.Error("finished a sign-in that was never started")
	}
}

func TestTheSiteIsAskedAboutItsSignInRarely(t *testing.T) {
	site := newOAuthSite(t)
	dir := t.TempDir()
	a := newTestCore(t, dir)
	// Knowing what is known asks nobody.
	if st := a.ConnectStatus(site.URL); st.Known || st.Available || site.asked != 0 {
		t.Fatalf("status before asking = %+v, asked %d", st, site.asked)
	}
	first := a.ConnectOffer(site.URL)
	again := a.ConnectOffer(site.URL)
	if !first.Available || !again.Available || site.asked != 1 {
		t.Fatalf("asked %d times for two calls: %+v %+v", site.asked, first, again)
	}
	if st := a.ConnectStatus(site.URL); !st.Known || !st.Available || site.asked != 1 {
		t.Errorf("status after asking = %+v, asked %d", st, site.asked)
	}
	// A restart remembers: no new request.
	a.bmMu.Lock()
	a.bookmarks.Close()
	a.bmMu.Unlock()
	b := newTestCore(t, dir)
	if st := b.ConnectStatus(site.URL); !st.Known || !st.Available || len(st.Scopes) != 2 {
		t.Errorf("after a restart: %+v", st)
	}
	if b.ConnectOffer(site.URL); site.asked != 1 {
		t.Errorf("asked again after a restart (%d)", site.asked)
	}
}

func TestSitesWithoutASignInAreRememberedToo(t *testing.T) {
	var asked int
	none := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { asked++; http.NotFound(w, r) }))
	defer none.Close()
	a := newTestCore(t, t.TempDir())
	for i := 0; i < 3; i++ {
		if o := a.ConnectOffer(none.URL); o.Available || !o.Known {
			t.Fatalf("offer = %+v", o)
		}
	}
	if asked != 2 { // the two well-known addresses, once
		t.Errorf("asked %d times", asked)
	}
	// A site that cannot be reached is a different answer: it is asked again soon, not tomorrow.
	gone := httptest.NewServer(http.NotFoundHandler())
	url := gone.URL
	gone.Close()
	o := a.ConnectOffer(url)
	if o.Available || !strings.Contains(o.Reason, "could not reach") {
		t.Fatalf("unreachable = %+v", o)
	}
	a.connects.mu.Lock()
	c := a.connects.offers[url]
	a.connects.mu.Unlock()
	if !c.Unreached || !c.fresh() {
		t.Errorf("the unreached answer = %+v", c)
	}
	c.At = time.Now().Add(-2 * time.Hour)
	if c.fresh() {
		t.Error("an answer of no contact was kept for hours")
	}
	n := cachedOffer{Reason: "none", At: time.Now().Add(-2 * time.Hour)}
	if !n.fresh() {
		t.Error("an answer of no sign-in was not kept for a day")
	}
	if (cachedOffer{Available: true, At: time.Now().Add(-6 * 24 * time.Hour)}).fresh() != true || (cachedOffer{Available: true, At: time.Now().Add(-8 * 24 * time.Hour)}).fresh() {
		t.Error("a sign-in that is offered should be kept for a week")
	}
}
