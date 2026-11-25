package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

// Alert is one message to post.
type Alert struct {
	Light         tracker.Light
	PriceGold     int64
	ThresholdGold int64
	Region        string
	UpdatedAt     time.Time
	Test          bool
}

const (
	colorGreen = 0x2ECC71
	colorRed   = 0xE74C3C
)

type embed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       int    `json:"color"`
	Timestamp   string `json:"timestamp,omitempty"`
	Footer      struct {
		Text string `json:"text"`
	} `json:"footer"`
}

type message struct {
	Content string  `json:"content,omitempty"`
	Embeds  []embed `json:"embeds"`
}

func build(a Alert) message {
	threshold := tracker.FormatGold(a.ThresholdGold)
	e := embed{Timestamp: a.UpdatedAt.UTC().Format(time.RFC3339)}
	e.Footer.Text = "WoW Token Tracker"
	region := strings.ToUpper(a.Region)
	if a.Light == tracker.Green {
		e.Title = "🟢 Under " + threshold + " — buy"
		e.Color = colorGreen
	} else {
		e.Title = "🔴 Back above " + threshold
		e.Color = colorRed
	}
	e.Description = fmt.Sprintf("WoW Token (%s) is **%s**. Threshold: %s.", region, tracker.FormatGold(a.PriceGold), threshold)
	m := message{Embeds: []embed{e}}
	if a.Test {
		m.Content = "Test message from WoW Token Tracker — your webhook works."
	}
	return m
}

// Send posts a to the webhook. Errors never include the webhook URL.
func Send(ctx context.Context, httpc *http.Client, webhookURL string, a Alert) error {
	body, err := json.Marshal(build(a))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL+"?wait=true", bytes.NewReader(body))
	if err != nil {
		return errors.New("webhook URL is not valid")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("can't reach Discord: %v", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return errors.New("Discord rate limited the webhook (429); will retry")
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("Discord webhook not found (%d) — was it deleted?", resp.StatusCode)
	default:
		return fmt.Errorf("Discord returned %d", resp.StatusCode)
	}
}
