package settingsui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/StormChild/Wow-Token-Tracker/internal/config"
)

const hook = "https://discord.com/api/webhooks/1/abc"

type rec struct {
	saved   *config.Settings
	tested  *config.Settings
	saveErr error
	testErr error
}

func (r *rec) callbacks() Callbacks {
	return Callbacks{
		Load: func() config.Settings {
			s := config.Defaults()
			s.ClientID, s.ClientSecret, s.WebhookURL = "cid", "S3CRET", hook
			return s
		},
		Save: func(s config.Settings) error { r.saved = &s; return r.saveErr },
		Test: func(_ context.Context, s config.Settings) (int64, error) {
			r.tested = &s
			return 395120, r.testErr
		},
	}
}

func do(t *testing.T, h http.Handler, method, target string, form url.Values, host string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Host = host
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

const local = "127.0.0.1:5555"

func formWith(token string, kv ...string) url.Values {
	v := url.Values{"t": {token}, "region": {"us"}, "clientId": {"cid"}, "clientSecret": {""},
		"webhookUrl": {""}, "thresholdGold": {"400000"}, "pollMinutes": {"20"}}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

func TestRejectsMissingToken(t *testing.T) {
	r := &rec{}
	h := NewHandler("tok", r.callbacks())
	if w := do(t, h, "GET", "/", nil, local); w.Code != http.StatusForbidden {
		t.Fatalf("GET no token: %d", w.Code)
	}
	if w := do(t, h, "GET", "/?t=wrong", nil, local); w.Code != http.StatusForbidden {
		t.Fatalf("GET wrong token: %d", w.Code)
	}
	if w := do(t, h, "POST", "/save", formWith("wrong"), local); w.Code != http.StatusForbidden || r.saved != nil {
		t.Fatalf("POST wrong token: %d saved=%v", w.Code, r.saved)
	}
}

func TestRejectsForeignHost(t *testing.T) {
	h := NewHandler("tok", (&rec{}).callbacks())
	if w := do(t, h, "GET", "/?t=tok", nil, "evil.com"); w.Code != http.StatusForbidden {
		t.Fatalf("foreign host: %d", w.Code)
	}
	if w := do(t, h, "GET", "/?t=tok", nil, "localhost:5555"); w.Code != http.StatusOK {
		t.Fatalf("localhost: %d", w.Code)
	}
}

func TestGetNeverEchoesSecrets(t *testing.T) {
	w := do(t, NewHandler("tok", (&rec{}).callbacks()), "GET", "/?t=tok", nil, local)
	body := w.Body.String()
	if w.Code != http.StatusOK || strings.Contains(body, "S3CRET") || strings.Contains(body, "webhooks/1/abc") {
		t.Fatalf("code %d; secrets echoed in body", w.Code)
	}
	if !strings.Contains(body, "saved — leave blank to keep") || !strings.Contains(body, `value="cid"`) {
		t.Fatal("form missing placeholder or client id")
	}
}

func TestSaveBlankSecretsKeepsExisting(t *testing.T) {
	r := &rec{}
	w := do(t, NewHandler("tok", r.callbacks()), "POST", "/save", formWith("tok", "thresholdGold", "380,000"), local)
	if r.saved == nil {
		t.Fatalf("not saved; code %d body %s", w.Code, w.Body)
	}
	if r.saved.ClientSecret != "S3CRET" || r.saved.WebhookURL != hook || r.saved.ThresholdGold != 380000 {
		t.Fatalf("saved = %+v", *r.saved)
	}
	if !strings.Contains(w.Body.String(), "Saved ✓") {
		t.Fatal("no Saved message")
	}
}

func TestSaveReplacesSecretWhenTyped(t *testing.T) {
	r := &rec{}
	do(t, NewHandler("tok", r.callbacks()), "POST", "/save", formWith("tok", "clientSecret", " NEW "), local)
	if r.saved == nil || r.saved.ClientSecret != "NEW" {
		t.Fatalf("saved = %+v", r.saved)
	}
}

func TestSaveShowsValidationError(t *testing.T) {
	r := &rec{saveErr: errors.New("check interval must be at least 5 minutes")}
	w := do(t, NewHandler("tok", r.callbacks()), "POST", "/save", formWith("tok", "pollMinutes", "2"), local)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "check interval must be at least 5 minutes") {
		t.Fatalf("code %d body %s", w.Code, w.Body)
	}
}

func TestSaveRejectsNonNumericThreshold(t *testing.T) {
	r := &rec{}
	w := do(t, NewHandler("tok", r.callbacks()), "POST", "/save", formWith("tok", "thresholdGold", "lots"), local)
	if r.saved != nil || !strings.Contains(w.Body.String(), "whole number") {
		t.Fatalf("saved=%v body %s", r.saved, w.Body)
	}
}

func TestTestButton(t *testing.T) {
	r := &rec{}
	w := do(t, NewHandler("tok", r.callbacks()), "POST", "/test", formWith("tok", "region", "eu"), local)
	if r.tested == nil || r.tested.Region != "eu" || r.tested.ClientSecret != "S3CRET" {
		t.Fatalf("tested = %+v", r.tested)
	}
	if r.saved != nil {
		t.Fatal("Test must not save")
	}
	if !strings.Contains(w.Body.String(), "Test sent — check Discord (price 395,120g)") {
		t.Fatalf("body %s", w.Body)
	}
}

func TestTestButtonShowsError(t *testing.T) {
	r := &rec{testErr: errors.New("Blizzard rejected the client ID or secret")}
	w := do(t, NewHandler("tok", r.callbacks()), "POST", "/test", formWith("tok"), local)
	if !strings.Contains(w.Body.String(), "Blizzard rejected the client ID or secret") {
		t.Fatalf("body %s", w.Body)
	}
}

func TestRegionsShownUppercase(t *testing.T) {
	w := do(t, NewHandler("tok", (&rec{}).callbacks()), "GET", "/?t=tok", nil, local)
	body := w.Body.String()
	if !strings.Contains(body, `<option value="us" selected>US</option>`) || !strings.Contains(body, `>TW</option>`) {
		t.Fatalf("region options not uppercase: %s", body)
	}
}

// store is a stateful Load/Save pair that starts empty, like a first run.
type store struct{ s config.Settings }

func (st *store) callbacks() Callbacks {
	st.s = config.Defaults()
	return Callbacks{
		Load: func() config.Settings { return st.s },
		Save: func(s config.Settings) error { st.s = s; return nil },
		Test: func(context.Context, config.Settings) (int64, error) { return 395120, nil },
	}
}

func TestTestThenSaveOnEmptyStoreKeepsTypedSecrets(t *testing.T) {
	st := &store{}
	h := NewHandler("tok", st.callbacks())
	w := do(t, h, "POST", "/test", formWith("tok", "clientSecret", "S3CRET", "webhookUrl", hook), local)
	if strings.Contains(w.Body.String(), "S3CRET") || strings.Contains(w.Body.String(), "webhooks/1/abc") {
		t.Fatal("secrets echoed after Test")
	}
	if !strings.Contains(w.Body.String(), "leave blank to keep") {
		t.Fatal("after Test the secret fields don't say they'll be kept")
	}
	do(t, h, "POST", "/save", formWith("tok"), local) // browser re-submits blank password fields
	if st.s.ClientSecret != "S3CRET" || st.s.WebhookURL != hook {
		t.Fatalf("typed secrets lost: %+v", st.s)
	}
}

func TestFailedSaveKeepsTypedSecrets(t *testing.T) {
	st := &store{}
	h := NewHandler("tok", st.callbacks())
	do(t, h, "POST", "/save", formWith("tok", "clientSecret", "S3CRET", "webhookUrl", hook, "thresholdGold", "lots"), local)
	do(t, h, "POST", "/save", formWith("tok"), local)
	if st.s.ClientSecret != "S3CRET" || st.s.WebhookURL != hook {
		t.Fatalf("typed secrets lost after failed save: %+v", st.s)
	}
}
