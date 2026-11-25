package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

type payload struct {
	Content string `json:"content"`
	Embeds  []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Color       int    `json:"color"`
		Timestamp   string `json:"timestamp"`
		Footer      struct {
			Text string `json:"text"`
		} `json:"footer"`
	} `json:"embeds"`
}

var updated = time.Date(2026, 11, 25, 14, 0, 0, 0, time.UTC)

func capture(t *testing.T, status int) (string, *payload, *string) {
	t.Helper()
	var got payload
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("method %s, content-type %q", r.Method, r.Header.Get("Content-Type"))
		}
		query = r.URL.RawQuery
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/api/webhooks/1/abc", &got, &query
}

func alert(l tracker.Light) Alert {
	return Alert{Light: l, PriceGold: 395120, ThresholdGold: 400000, Region: "us", UpdatedAt: updated}
}

func TestSendGreen(t *testing.T) {
	url, got, query := capture(t, http.StatusOK)
	if err := Send(context.Background(), http.DefaultClient, url, alert(tracker.Green)); err != nil {
		t.Fatal(err)
	}
	if *query != "wait=true" {
		t.Fatalf("query = %q", *query)
	}
	if len(got.Embeds) != 1 {
		t.Fatalf("embeds = %d", len(got.Embeds))
	}
	e := got.Embeds[0]
	if !strings.Contains(e.Title, "Under 400,000g — buy") {
		t.Errorf("title = %q", e.Title)
	}
	if !strings.Contains(e.Description, "395,120g") || !strings.Contains(e.Description, "US") {
		t.Errorf("description = %q", e.Description)
	}
	if e.Color != 0x2ECC71 {
		t.Errorf("color = %#x", e.Color)
	}
	if e.Timestamp != "2026-11-25T14:00:00Z" {
		t.Errorf("timestamp = %q", e.Timestamp)
	}
	if e.Footer.Text != "WoW Token Tracker" {
		t.Errorf("footer = %q", e.Footer.Text)
	}
	if strings.Contains(got.Content, "Test message") {
		t.Errorf("non-test alert has test content: %q", got.Content)
	}
}

func TestSendRed(t *testing.T) {
	url, got, _ := capture(t, http.StatusNoContent)
	a := alert(tracker.Red)
	a.PriceGold = 410000
	if err := Send(context.Background(), http.DefaultClient, url, a); err != nil {
		t.Fatal(err)
	}
	e := got.Embeds[0]
	if !strings.Contains(e.Title, "Back above 400,000g") {
		t.Errorf("title = %q", e.Title)
	}
	if e.Color != 0xE74C3C {
		t.Errorf("color = %#x", e.Color)
	}
}

func TestSendTest(t *testing.T) {
	url, got, _ := capture(t, http.StatusOK)
	a := alert(tracker.Green)
	a.Test = true
	if err := Send(context.Background(), http.DefaultClient, url, a); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "Test message") {
		t.Errorf("content = %q", got.Content)
	}
}

func TestSend404(t *testing.T) {
	url, _, _ := capture(t, http.StatusNotFound)
	err := Send(context.Background(), http.DefaultClient, url, alert(tracker.Green))
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "abc") {
		t.Fatalf("error leaks webhook token: %v", err)
	}
}

func TestSend429(t *testing.T) {
	url, _, _ := capture(t, http.StatusTooManyRequests)
	err := Send(context.Background(), http.DefaultClient, url, alert(tracker.Green))
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnreachableDoesNotLeakWebhook(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL + "/api/webhooks/1/SECRETTOKEN"
	srv.Close()
	err := Send(context.Background(), http.DefaultClient, url, alert(tracker.Green))
	if err == nil || strings.Contains(err.Error(), "SECRETTOKEN") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnreachableIsPlainEnglish(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL + "/api/webhooks/1/x"
	srv.Close()
	err := Send(context.Background(), http.DefaultClient, url, alert(tracker.Green))
	if err == nil || err.Error() != "can't reach Discord — check your internet connection" {
		t.Fatalf("err = %v", err)
	}
}
