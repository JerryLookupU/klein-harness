package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStatusPrefersEnvKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-key")
	status, err := LoadStatus(t.TempDir())
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if !status.Authenticated || status.Source != "env:OPENAI_API_KEY" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSaveAPIKeyAndLoadStatus(t *testing.T) {
	home := t.TempDir()
	path, err := SaveAPIKey(home, "stored-key")
	if err != nil {
		t.Fatalf("save api key: %v", err)
	}
	if filepath.Dir(path) != home {
		t.Fatalf("unexpected auth path: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat auth file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected file mode: %o", info.Mode().Perm())
	}
	status, err := LoadStatus(home)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if !status.Authenticated || status.Source != path || status.Mode != "api_key" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestLoadStatusFromCodexChatGPTAuthFile(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{
  "auth_mode": "chatgpt",
  "OPENAI_API_KEY": null,
  "tokens": {"id_token":"x","refresh_token":"y"},
  "last_refresh": "2026-03-24T08:00:00Z"
}`), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}
	status, err := LoadStatus(home)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if !status.Authenticated || status.Mode != "chatgpt" || status.Provider != "openai" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestLoadStatusFromCodexAPIKeyAuthFile(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{
  "auth_mode": "apikey",
  "OPENAI_API_KEY": "sk-test",
  "tokens": null,
  "last_refresh": "2026-03-24T08:00:00Z"
}`), 0o600); err != nil {
		t.Fatalf("write auth file: %v", err)
	}
	status, err := LoadStatus(home)
	if err != nil {
		t.Fatalf("load status: %v", err)
	}
	if !status.Authenticated || status.Mode != "api_key" || status.Provider != "openai" {
		t.Fatalf("unexpected status: %+v", status)
	}
}
