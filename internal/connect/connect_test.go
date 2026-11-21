package connect

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// site is a server that follows the standards the package uses.
type site struct {
	*httptest.Server
	mu         sync.Mutex
	codes      map[string]string // code to challenge
	refreshes  map[string]bool
	registered int
	issued     int
	revoked    []string
	noRefresh  bool // a refresh answer without a new refresh token
	meta       func(map[string]any)
	denyReg    bool
	lastReg    map[string]any // what the last registration said about the app
}

func newSite(t *testing.T) *site {
	s := &site{codes: map[string]string{}, refreshes: map[string]bool{}}
	mux := http.NewServeMux()
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		m := map[string]any{
			"issuer": s.URL, "authorization_endpoint": s.URL + "/oauth/authorize", "token_endpoint": s.URL + "/oauth/token",
			"registration_endpoint": s.URL + "/oauth/register", "revocation_endpoint": s.URL + "/oauth/revoke",
			"code_challenge_methods_supported": []string{"S256"}, "grant_types_supported": []string{"authorization_code", "refresh_token"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_post"}, "scopes_supported": []string{"a", "b"},
		}
		if s.meta != nil {
			s.meta(m)
		}
		json.NewEncoder(w).Encode(m)
	})
	mux.HandleFunc("/oauth/register", func(w http.ResponseWriter, r *http.Request) {
		if s.denyReg {
			http.Error(w, `{"error":"invalid_redirect_uri"}`, 400)
			return
		}
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		s.mu.Lock()
		s.registered++
		s.lastReg = in
		s.mu.Unlock()
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
		bad := func() { w.WriteHeader(400); io.WriteString(w, `{"error":"invalid_grant"}`) }
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			ch, ok := s.codes[r.Form.Get("code")]
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != ch || r.Form.Get("client_secret") != "sec" {
				bad()
				return
			}
			delete(s.codes, r.Form.Get("code"))
		case "refresh_token":
			if !s.refreshes[r.Form.Get("refresh_token")] {
				bad()
				return
			}
			delete(s.refreshes, r.Form.Get("refresh_token")) // rotation: the old one is used up
		}
		s.issued++ // a counter: the clock on Windows can give the same time twice
		access, refresh := fmt.Sprintf("at-%d", s.issued), fmt.Sprintf("rt-%d", s.issued)
		out := map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 3600, "scope": "a b"}
		if !(s.noRefresh && r.Form.Get("grant_type") == "refresh_token") {
			out["refresh_token"] = refresh
			s.refreshes[refresh] = true
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/oauth/revoke", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		s.mu.Lock()
		s.revoked = append(s.revoked, r.Form.Get("token"))
		s.mu.Unlock()
	})
	return s
}

// signIn plays the reader: opens the address the sign-in gave, which ends in the browser coming back.
func signIn(address string) {
	go func() {
		if resp, err := http.Get(address); err == nil {
			resp.Body.Close()
		}
	}()
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestSignInEndToEnd(t *testing.T) {
	s := newSite(t)
	ctx := context.Background()
	sess, err := Begin(ctx, http.DefaultClient, s.URL, "a b", nil, freePort(t))
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if sess.Client().ID != "cid" || s.registered != 1 {
		t.Fatalf("client %+v, registered %d", sess.Client(), s.registered)
	}
	for _, want := range []string{"code_challenge_method=S256", "response_type=code", "client_id=cid", "state=", "scope=a+b"} {
		if !strings.Contains(sess.AuthURL, want) {
			t.Errorf("the sign-in address lacks %q: %s", want, sess.AuthURL)
		}
	}
	signIn(sess.AuthURL)
	tok, err := sess.Wait(ctx)
	if err != nil || tok.Access == "" || tok.Refresh == "" || tok.Scope != "a b" {
		t.Fatalf("tokens %+v, %v", tok, err)
	}
	if d := time.Until(tok.ExpiresAt); d < 59*time.Minute || d > time.Hour {
		t.Errorf("expires in %v", d)
	}
}

func TestAKeptClientIsUsedForItsOwnAddress(t *testing.T) {
	s := newSite(t)
	ctx := context.Background()
	port := freePort(t)
	first, err := Begin(ctx, http.DefaultClient, s.URL, "a", nil, port)
	if err != nil {
		t.Fatal(err)
	}
	kept := first.Client()
	first.Close()
	again, err := Begin(ctx, http.DefaultClient, s.URL, "a", &kept, port)
	if err != nil {
		t.Fatal(err)
	}
	again.Close()
	if s.registered != 1 {
		t.Errorf("registered %d times; the kept client should have been used", s.registered)
	}
	// The port is taken by another program: another is used, and the client for the other address is not.
	taken, _ := net.Listen("tcp", "127.0.0.1:0")
	defer taken.Close()
	busy, err := Begin(ctx, http.DefaultClient, s.URL, "a", &kept, taken.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if s.registered != 2 || busy.Client().Redirect == kept.Redirect {
		t.Errorf("a busy port: registered %d, redirect %s", s.registered, busy.Client().Redirect)
	}
}

func TestAnAnswerOfAnotherSignInDoesNotEndThisOne(t *testing.T) {
	s := newSite(t)
	ctx := context.Background()
	sess, err := Begin(ctx, http.DefaultClient, s.URL, "a", nil, freePort(t))
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	u, _ := url.Parse(sess.AuthURL)
	redirect := u.Query().Get("redirect_uri")
	resp, err := http.Get(redirect + "?code=stolen&state=wrong")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("a stray answer got %d", resp.StatusCode)
	}
	signIn(sess.AuthURL) // the real one still works
	if _, err := sess.Wait(ctx); err != nil {
		t.Errorf("the sign-in was spoiled: %v", err)
	}
}

func TestDenied(t *testing.T) {
	s := newSite(t)
	ctx := context.Background()
	sess, _ := Begin(ctx, http.DefaultClient, s.URL, "a", nil, freePort(t))
	defer sess.Close()
	u, _ := url.Parse(sess.AuthURL)
	go func() {
		resp, err := http.Get(u.Query().Get("redirect_uri") + "?error=access_denied&state=" + url.QueryEscape(u.Query().Get("state")))
		if err == nil {
			resp.Body.Close()
		}
	}()
	if _, err := sess.Wait(ctx); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("err = %v", err)
	}
}

func TestCancelledSignIn(t *testing.T) {
	s := newSite(t)
	ctx, cancel := context.WithCancel(context.Background())
	sess, _ := Begin(ctx, http.DefaultClient, s.URL, "a", nil, freePort(t))
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	if _, err := sess.Wait(ctx); err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("err = %v", err)
	}
}

func TestDiscoveryRefusals(t *testing.T) {
	for name, tweak := range map[string]func(map[string]any){
		"no S256":          func(m map[string]any) { m["code_challenge_methods_supported"] = []string{"plain"} },
		"no code flow":     func(m map[string]any) { m["grant_types_supported"] = []string{"client_credentials"} },
		"another origin":   func(m map[string]any) { m["token_endpoint"] = "https://elsewhere.example/token" },
		"no authorization": func(m map[string]any) { delete(m, "authorization_endpoint") },
	} {
		s := newSite(t)
		s.meta = tweak
		if _, err := Discover(context.Background(), http.DefaultClient, s.URL); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	empty := httptest.NewServer(http.NotFoundHandler())
	defer empty.Close()
	if _, err := Discover(context.Background(), http.DefaultClient, empty.URL); err == nil {
		t.Error("a site with no metadata was accepted")
	}
}

func TestRegistrationRefused(t *testing.T) {
	s := newSite(t)
	s.denyReg = true
	if _, err := Begin(context.Background(), http.DefaultClient, s.URL, "a", nil, freePort(t)); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("err = %v", err)
	}
}

func TestRefreshRotatesAndKeepsWhatIsNotReplaced(t *testing.T) {
	s := newSite(t)
	ctx := context.Background()
	sess, _ := Begin(ctx, http.DefaultClient, s.URL, "a", nil, freePort(t))
	signIn(sess.AuthURL)
	tok, err := sess.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client := sess.Client()
	next, err := Refresh(ctx, http.DefaultClient, s.URL+"/oauth/token", client, tok.Refresh)
	if err != nil || next.Access == tok.Access || next.Refresh == tok.Refresh {
		t.Fatalf("refresh: %+v, %v", next, err)
	}
	// The old refresh token is used up (the site rotates): using it again ends the sign-in.
	if _, err := Refresh(ctx, http.DefaultClient, s.URL+"/oauth/token", client, tok.Refresh); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("old refresh token: %v", err)
	}
	// A site that does not replace the refresh token: the old one is kept.
	s.noRefresh = true
	kept, err := Refresh(ctx, http.DefaultClient, s.URL+"/oauth/token", client, next.Refresh)
	if err != nil || kept.Refresh != next.Refresh {
		t.Errorf("not replaced: %+v, %v", kept, err)
	}
}

func TestRevoke(t *testing.T) {
	s := newSite(t)
	if err := Revoke(context.Background(), http.DefaultClient, s.URL+"/oauth/revoke", Client{ID: "cid", Secret: "sec"}, "tok"); err != nil {
		t.Fatal(err)
	}
	if len(s.revoked) != 1 || s.revoked[0] != "tok" {
		t.Errorf("revoked %v", s.revoked)
	}
}

func TestRegistrationTellsTheSiteWhoMardoIs(t *testing.T) {
	s := newSite(t)
	sess, err := Begin(context.Background(), http.DefaultClient, s.URL, "a", nil, freePort(t))
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	for k, want := range map[string]string{"client_name": "Mardo", "client_uri": ClientURI, "logo_uri": LogoURI, "policy_uri": PolicyURI} {
		if s.lastReg[k] != want {
			t.Errorf("%s = %v, want %s", k, s.lastReg[k], want)
		}
	}
	if !strings.HasPrefix(LogoURI, "https://") {
		t.Errorf("the logo must be an https address: %s", LogoURI)
	}
}

func TestAClientFromAnOlderRegistrationIsRegisteredAgain(t *testing.T) {
	s := newSite(t)
	port := freePort(t)
	old := Client{ID: "old", Secret: "old", Redirect: "http://127.0.0.1:" + strconv.Itoa(port) + "/callback"} // no version: made before the logo
	sess, err := Begin(context.Background(), http.DefaultClient, s.URL, "a", &old, port)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	if s.registered != 1 || sess.Client().ID == "old" {
		t.Errorf("registered %d times, client %s", s.registered, sess.Client().ID)
	}
	if sess.Client().Version != registrationVersion {
		t.Errorf("version %d", sess.Client().Version)
	}
}

func TestTheAnswerPage(t *testing.T) {
	p := answerPage(true, "<b>x</b>.example")
	for _, want := range []string{"data:image/png;base64,", "Close this window", "Back in Mardo", "&lt;b&gt;x"} {
		if !strings.Contains(p, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	if strings.Contains(p, "<b>x") || strings.Contains(p, "%!") {
		t.Error("the host is not escaped, or the template is broken")
	}
	if !strings.Contains(answerPage(false, "h"), "Not signed in") {
		t.Error("the refusal page")
	}
}
