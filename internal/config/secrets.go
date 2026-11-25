package config

// SecretStore holds values that must never be written to settings.json.
type SecretStore interface {
	Get(name string) (string, error) // missing → "", nil
	Set(name, value string) error
	Delete(name string) error // missing → nil
}

const (
	SecretClientSecret = "WowTokenTracker/clientSecret"
	SecretWebhookURL   = "WowTokenTracker/webhookUrl"
)

// MemSecrets is an in-memory SecretStore for tests.
type MemSecrets map[string]string

func (m MemSecrets) Get(name string) (string, error) { return m[name], nil }
func (m MemSecrets) Set(name, value string) error    { m[name] = value; return nil }
func (m MemSecrets) Delete(name string) error        { delete(m, name); return nil }
