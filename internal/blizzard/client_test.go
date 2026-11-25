package blizzard

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fake struct {
	oauthHits   atomic.Int32
	priceHits   atomic.Int32
	oauthStatus int
	priceFirst  int // status for the first price call; 0 = 200
	priceStatus int // status for later calls; 0 = 200
	t           *testing.T
}

func (f *fake) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		f.oauthHits.Add(1)
		if f.oauthStatus != 0 {
			w.WriteHeader(f.oauthStatus)
			return
		}
		id, secret, ok := r.BasicAuth()
		if !ok || id != "id" || secret != "secret" {
			f.t.Errorf("basic auth = %q:%q ok=%v", id, secret, ok)
		}
		if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "client_credentials" {
			f.t.Errorf("grant_type = %q", r.PostForm.Get("grant_type"))
		}
		fmt.Fprint(w, `{"access_token":"abc","token_type":"bearer","expires_in":86399}`)
	})
	mux.HandleFunc("/data/wow/token/index", func(w http.ResponseWriter, r *http.Request) {
		n := f.priceHits.Add(1)
		if n == 1 && f.priceFirst != 0 {
			w.WriteHeader(f.priceFirst)
			return
		}
		if f.priceStatus != 0 {
			w.WriteHeader(f.priceStatus)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer abc" {
			f.t.Errorf("Authorization = %q", got)
		}
		q := r.URL.Query()
		if q.Get("namespace") != "dynamic-us" || q.Get("locale") != "en_US" {
			f.t.Errorf("query = %v", q)
		}
		fmt.Fprint(w, `{"_links":{},"last_updated_timestamp":1764072000000,"price":3951200000}`)
	})
	return mux
}

func setup(t *testing.T, f *fake) *Client {
	t.Helper()
	f.t = t
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	c := New("id", "secret")
	c.OAuthURL = srv.URL + "/token"
	c.APIBaseFmt = srv.URL + "%.0s" // swallow the region
	return c
}

func TestTokenPrice(t *testing.T) {
	c := setup(t, &fake{})
	obs, err := c.TokenPrice(context.Background(), "us")
	if err != nil {
		t.Fatal(err)
	}
	if obs.PriceCopper != 3951200000 {
		t.Fatalf("price = %d", obs.PriceCopper)
	}
	if !obs.UpdatedAt.Equal(time.UnixMilli(1764072000000)) {
		t.Fatalf("updated = %v", obs.UpdatedAt)
	}
}

func TestTokenCached(t *testing.T) {
	f := &fake{}
	c := setup(t, f)
	for range 2 {
		if _, err := c.TokenPrice(context.Background(), "us"); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.oauthHits.Load(); n != 1 {
		t.Fatalf("oauth hits = %d, want 1", n)
	}
}

func TestRefreshOn401(t *testing.T) {
	f := &fake{priceFirst: http.StatusUnauthorized}
	c := setup(t, f)
	if _, err := c.TokenPrice(context.Background(), "us"); err != nil {
		t.Fatal(err)
	}
	if n := f.oauthHits.Load(); n != 2 {
		t.Fatalf("oauth hits = %d, want 2", n)
	}
}

func TestBadCredentials(t *testing.T) {
	c := setup(t, &fake{oauthStatus: http.StatusUnauthorized})
	_, err := c.TokenPrice(context.Background(), "us")
	if !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("err = %v, want ErrBadCredentials", err)
	}
}

func TestServerError(t *testing.T) {
	c := setup(t, &fake{priceStatus: http.StatusServiceUnavailable})
	_, err := c.TokenPrice(context.Background(), "us")
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err = %v, want mention of 503", err)
	}
	if strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaks secret: %v", err)
	}
}

func TestPersistent401IsBadCredentials(t *testing.T) {
	c := setup(t, &fake{priceStatus: http.StatusUnauthorized})
	if _, err := c.TokenPrice(context.Background(), "us"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("err = %v, want ErrBadCredentials", err)
	}
}
