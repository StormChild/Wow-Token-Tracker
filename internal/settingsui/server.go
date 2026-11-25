package settingsui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/StormChild/Wow-Token-Tracker/internal/config"
	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

// Callbacks connect the page to the app.
type Callbacks struct {
	Load func() config.Settings
	Save func(config.Settings) error                                 // validates and persists
	Test func(ctx context.Context, s config.Settings) (int64, error) // fetch price (gold) + send test alert
}

//go:embed page.html
var pageHTML string

var page = template.Must(template.New("page").Funcs(template.FuncMap{"upper": strings.ToUpper}).Parse(pageHTML))

type view struct {
	S          config.Settings
	Regions    []string
	Token      string
	HasSecret  bool
	HasWebhook bool
	Msg        string
	IsErr      bool
}

type handler struct {
	token string
	cb    Callbacks

	// Secrets typed but not yet saved (after a Test or a failed Save). The page never echoes
	// secrets, so the browser re-submits those fields blank; these fill them in until a Save succeeds.
	mu             sync.Mutex
	pendingSecret  string
	pendingWebhook string
}

// NewHandler serves the settings page; every request must carry token and a loopback Host.
func NewHandler(token string, cb Callbacks) http.Handler {
	h := &handler{token: token, cb: cb}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", h.get)
	mux.HandleFunc("POST /save", h.save)
	mux.HandleFunc("POST /test", h.test)
	return h.guard(mux)
}

func (h *handler) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.FormValue("t")), []byte(h.token)) != 1 {
			http.Error(w, "forbidden — open Settings from the tray menu", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (h *handler) render(w http.ResponseWriter, s config.Settings, msg string, isErr bool) {
	stored := h.cb.Load()
	h.mu.Lock()
	hasSecret := stored.ClientSecret != "" || h.pendingSecret != ""
	hasWebhook := stored.WebhookURL != "" || h.pendingWebhook != ""
	h.mu.Unlock()
	v := view{
		S: s, Regions: config.Regions, Token: h.token,
		HasSecret: hasSecret, HasWebhook: hasWebhook,
		Msg: msg, IsErr: isErr,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, v); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	h.render(w, h.cb.Load(), "", false)
}

// merge reads the form over the stored settings. A blank secret field keeps the value typed
// earlier in this session, else the stored one. Typed secrets are remembered until a Save succeeds.
func (h *handler) merge(r *http.Request) (config.Settings, error) {
	s := h.cb.Load()
	s.Region = strings.TrimSpace(r.PostFormValue("region"))
	s.ClientID = strings.TrimSpace(r.PostFormValue("clientId"))
	h.mu.Lock()
	if v := strings.TrimSpace(r.PostFormValue("clientSecret")); v != "" {
		h.pendingSecret = v
	}
	if v := strings.TrimSpace(r.PostFormValue("webhookUrl")); v != "" {
		h.pendingWebhook = v
	}
	if h.pendingSecret != "" {
		s.ClientSecret = h.pendingSecret
	}
	if h.pendingWebhook != "" {
		s.WebhookURL = h.pendingWebhook
	}
	h.mu.Unlock()
	var errs []error
	th, err := parseWhole(r.PostFormValue("thresholdGold"))
	if err != nil {
		errs = append(errs, errors.New("threshold must be a whole number of gold, e.g. 400000"))
	}
	s.ThresholdGold = th
	poll, err := parseWhole(r.PostFormValue("pollMinutes"))
	if err != nil {
		errs = append(errs, errors.New("check interval must be a whole number of minutes"))
	}
	s.PollMinutes = int(poll)
	return s, errors.Join(errs...)
}

func parseWhole(v string) (int64, error) {
	v = strings.NewReplacer(",", "", " ", "", "g", "", "G", "").Replace(v)
	return strconv.ParseInt(v, 10, 64)
}

func (h *handler) save(w http.ResponseWriter, r *http.Request) {
	s, err := h.merge(r)
	if err == nil {
		err = h.cb.Save(s)
	}
	if err != nil {
		h.render(w, s, err.Error(), true)
		return
	}
	h.mu.Lock()
	h.pendingSecret, h.pendingWebhook = "", ""
	h.mu.Unlock()
	h.render(w, s, "Saved ✓ — the tray icon will update in a few seconds.", false)
}

func (h *handler) test(w http.ResponseWriter, r *http.Request) {
	s, err := h.merge(r)
	if err == nil {
		err = s.Validate()
	}
	if err == nil && !s.Complete() {
		err = errors.New("fill in Client ID, Client Secret and Discord Webhook URL first")
	}
	if err != nil {
		h.render(w, s, err.Error(), true)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	gold, err := h.cb.Test(ctx, s)
	if err != nil {
		h.render(w, s, err.Error(), true)
		return
	}
	h.render(w, s, fmt.Sprintf("Test sent — check Discord (price %s). Click Save to keep these settings.", tracker.FormatGold(gold)), false)
}

// Server is the running settings page.
type Server struct {
	srv   *http.Server
	url   string
	token string
}

// Start listens on a random loopback port with a fresh random token.
func Start(cb Callbacks) (*Server, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(b)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{
		srv:   &http.Server{Handler: NewHandler(token, cb), ReadHeaderTimeout: 10 * time.Second},
		url:   fmt.Sprintf("http://%s/?t=%s", ln.Addr().String(), token),
		token: token,
	}
	go s.srv.Serve(ln)
	return s, nil
}

func (s *Server) URL() string  { return s.url }
func (s *Server) Close() error { return s.srv.Close() }
