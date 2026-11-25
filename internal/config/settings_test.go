package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testHook = "https://discord.com/api/webhooks/1/abcDEF"

func newStore(t *testing.T) Store {
	t.Helper()
	return Store{Dir: t.TempDir(), Secrets: MemSecrets{}}
}

func TestLoadMissingReturnsDefaults(t *testing.T) {
	s, err := newStore(t).Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.Region != "us" || s.ThresholdGold != 400000 || s.PollMinutes != 20 {
		t.Fatalf("defaults wrong: %+v", s)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	st := newStore(t)
	in := Settings{Region: "eu", ClientID: "cid", ClientSecret: "S3CRET", WebhookURL: testHook, ThresholdGold: 380000, PollMinutes: 30}
	if err := st.Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip: got %+v want %+v", out, in)
	}
	raw, err := os.ReadFile(filepath.Join(st.Dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "S3CRET") || strings.Contains(string(raw), "webhooks") {
		t.Fatalf("secret leaked into settings.json: %s", raw)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	st := newStore(t)
	bad := Defaults()
	bad.Region = "cn"
	bad.ClientSecret = "S3CRET"
	if err := st.Save(bad); err == nil {
		t.Fatal("expected validation error")
	}
	if v, _ := st.Secrets.Get(SecretClientSecret); v != "" {
		t.Fatal("secret written despite invalid settings")
	}
}

func writeRaw(t *testing.T, st Store, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(st.Dir, "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadClampsPoll(t *testing.T) {
	st := newStore(t)
	writeRaw(t, st, `{"region":"us","clientId":"x","thresholdGold":400000,"pollMinutes":1}`)
	s, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.PollMinutes != 5 {
		t.Fatalf("poll = %d, want 5", s.PollMinutes)
	}
}

func TestLoadUnknownRegionFallsBackToUS(t *testing.T) {
	st := newStore(t)
	writeRaw(t, st, `{"region":"cn","thresholdGold":400000,"pollMinutes":20}`)
	s, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if s.Region != "us" {
		t.Fatalf("region = %q, want us", s.Region)
	}
}

func TestValidate(t *testing.T) {
	good := Defaults()
	good.WebhookURL = testHook
	if err := good.Validate(); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	alt := good
	alt.WebhookURL = "https://discordapp.com/api/webhooks/1/x"
	if err := alt.Validate(); err != nil {
		t.Fatalf("discordapp.com webhook rejected: %v", err)
	}
	mutations := map[string]func(*Settings){
		"region cn":    func(s *Settings) { s.Region = "cn" },
		"threshold 0":  func(s *Settings) { s.ThresholdGold = 0 },
		"poll 4":       func(s *Settings) { s.PollMinutes = 4 },
		"foreign hook": func(s *Settings) { s.WebhookURL = "https://evil.example/" },
		"http not tls": func(s *Settings) { s.WebhookURL = "http://discord.com/api/webhooks/1/x" },
	}
	for name, mut := range mutations {
		s := good
		mut(&s)
		if s.Validate() == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestComplete(t *testing.T) {
	full := Defaults()
	full.ClientID, full.ClientSecret, full.WebhookURL = "id", "secret", testHook
	if !full.Complete() {
		t.Fatal("full settings not complete")
	}
	for name, mut := range map[string]func(*Settings){
		"no id":      func(s *Settings) { s.ClientID = "" },
		"no secret":  func(s *Settings) { s.ClientSecret = "" },
		"no webhook": func(s *Settings) { s.WebhookURL = "" },
	} {
		s := full
		mut(&s)
		if s.Complete() {
			t.Errorf("%s: Complete() = true", name)
		}
	}
}
