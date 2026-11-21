package auth

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func oauthCred(expires time.Duration) Credential {
	return Credential{Origin: "https://site.test", Kind: Bearer, Token: "old-access", OAuth: &OAuthInfo{
		ClientID: "cid", ClientSecret: "sec", Redirect: "http://127.0.0.1:8765/callback", TokenURL: "https://site.test/oauth/token",
		RefreshToken: "old-refresh", ExpiresAt: time.Now().Add(expires), Scope: "a b"}}
}

func bearerOf(m *Manager) string {
	req, _ := http.NewRequest("GET", "https://site.test/x", nil)
	m.Apply(req)
	return req.Header.Get("Authorization")
}

func TestATokenThatIsAboutToEndIsRenewedOnceAndKept(t *testing.T) {
	m := NewManager(&MemorySecrets{}, "")
	var calls atomic.Int32
	m.SetRefresher(func(ctx context.Context, c Credential) (Credential, error) {
		calls.Add(1)
		time.Sleep(30 * time.Millisecond) // requests pile up meanwhile
		if c.OAuth.RefreshToken != "old-refresh" {
			t.Errorf("renewed with %q", c.OAuth.RefreshToken)
		}
		next := c
		o := *c.OAuth
		o.RefreshToken, o.ExpiresAt = "new-refresh", time.Now().Add(time.Hour)
		next.Token, next.OAuth = "new-access", &o
		return next, nil
	})
	if _, err := m.Save(oauthCred(20 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	got := make([]string, 8)
	for i := range got {
		wg.Add(1)
		go func() { defer wg.Done(); got[i] = bearerOf(m) }()
	}
	wg.Wait()
	for i, g := range got {
		if g != "Bearer new-access" {
			t.Errorf("request %d got %q", i, g)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("renewed %d times; the refresh token is replaced each time, so once", calls.Load())
	}
	if c, _ := m.Get("https://site.test"); c.OAuth.RefreshToken != "new-refresh" || c.Token != "new-access" {
		t.Errorf("the new tokens were not kept: %+v", c.OAuth)
	}
	if l := m.List(); len(l) != 1 || !l[0].OAuth || l[0].Scope != "a b" {
		t.Errorf("list = %+v", l)
	}
	bearerOf(m) // still fresh: no more renewals
	if calls.Load() != 1 {
		t.Errorf("renewed again while fresh")
	}
}

func TestATokenThatIsNotAboutToEndIsLeftAlone(t *testing.T) {
	m := NewManager(&MemorySecrets{}, "")
	m.SetRefresher(func(context.Context, Credential) (Credential, error) {
		t.Error("renewed too early")
		return Credential{}, nil
	})
	m.Save(oauthCred(time.Hour))
	if got := bearerOf(m); got != "Bearer old-access" {
		t.Errorf("got %q", got)
	}
	// A token typed in by hand has nothing to renew.
	m2 := NewManager(&MemorySecrets{}, "")
	m2.SetRefresher(func(context.Context, Credential) (Credential, error) {
		t.Error("renewed a typed token")
		return Credential{}, nil
	})
	m2.Save(Credential{Origin: "https://site.test", Kind: Bearer, Token: "typed"})
	if got := bearerOf(m2); got != "Bearer typed" {
		t.Errorf("got %q", got)
	}
}

func TestARefusedRenewalIsNotRepeatedAtEveryRequest(t *testing.T) {
	m := NewManager(&MemorySecrets{}, "")
	var calls atomic.Int32
	m.SetRefresher(func(context.Context, Credential) (Credential, error) {
		calls.Add(1)
		return Credential{}, errors.New("invalid_grant")
	})
	m.Save(oauthCred(-time.Minute)) // already ended
	for i := 0; i < 5; i++ {
		if got := bearerOf(m); got != "Bearer old-access" { // the old token goes: the site's answer is what the reader sees
			t.Errorf("got %q", got)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("tried %d times in a row", calls.Load())
	}
}

func TestKeptClients(t *testing.T) {
	m := NewManager(&MemorySecrets{}, "")
	if _, ok := m.Client("https://site.test"); ok {
		t.Error("a client from nowhere")
	}
	if err := m.SaveClient("https://site.test", []byte(`{"id":"cid"}`)); err != nil {
		t.Fatal(err)
	}
	if got, ok := m.Client("https://site.test"); !ok || string(got) != `{"id":"cid"}` {
		t.Errorf("client = %q, %v", got, ok)
	}
}
