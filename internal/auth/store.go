package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const envAPIKey = "OPENAI_API_KEY"

type FileCredential struct {
	Provider string `json:"provider"`
	APIKey   string `json:"apiKey"`
	SavedAt  string `json:"savedAt"`
}

type CodexAuthFile struct {
	AuthMode     string          `json:"auth_mode"`
	OpenAIAPIKey *string         `json:"OPENAI_API_KEY"`
	Tokens       json.RawMessage `json:"tokens"`
	LastRefresh  string          `json:"last_refresh"`
}

type Status struct {
	Authenticated bool   `json:"authenticated"`
	Mode          string `json:"mode"`
	Source        string `json:"source"`
	Provider      string `json:"provider"`
}

func SaveAPIKey(homeDir, apiKey string) (string, error) {
	if apiKey == "" {
		return "", errors.New("api key is empty")
	}
	dir := codexHome(homeDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "auth.json")
	key := apiKey
	payload, err := json.MarshalIndent(CodexAuthFile{
		AuthMode:     "apikey",
		OpenAIAPIKey: &key,
		LastRefresh:  time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func LoadStatus(homeDir string) (Status, error) {
	if value := os.Getenv(envAPIKey); value != "" {
		return Status{
			Authenticated: true,
			Mode:          "api_key",
			Source:        "env:OPENAI_API_KEY",
			Provider:      "openai",
		}, nil
	}
	path := filepath.Join(codexHome(homeDir), "auth.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Status{Authenticated: false, Mode: "none", Source: "none", Provider: "none"}, nil
		}
		return Status{}, err
	}
	status, ok, err := loadCodexAuthStatus(path, payload)
	if err != nil {
		return Status{}, err
	}
	if ok {
		return status, nil
	}
	status, ok, err = loadLegacyAuthStatus(path, payload)
	if err != nil {
		return Status{}, err
	}
	if ok {
		return status, nil
	}
	return Status{Authenticated: false, Mode: "none", Source: "none", Provider: "none"}, nil
}

func loadCodexAuthStatus(path string, payload []byte) (Status, bool, error) {
	var cred CodexAuthFile
	if err := json.Unmarshal(payload, &cred); err != nil {
		return Status{}, false, nil
	}
	if cred.AuthMode == "" && cred.OpenAIAPIKey == nil && len(cred.Tokens) == 0 {
		return Status{}, false, nil
	}
	switch {
	case cred.OpenAIAPIKey != nil && *cred.OpenAIAPIKey != "":
		return Status{
			Authenticated: true,
			Mode:          "api_key",
			Source:        path,
			Provider:      "openai",
		}, true, nil
	case cred.AuthMode == "chatgpt" && hasStructuredTokens(cred.Tokens):
		return Status{
			Authenticated: true,
			Mode:          "chatgpt",
			Source:        path,
			Provider:      "openai",
		}, true, nil
	case cred.AuthMode != "":
		return Status{
			Authenticated: false,
			Mode:          cred.AuthMode,
			Source:        path,
			Provider:      "openai",
		}, true, nil
	default:
		return Status{Authenticated: false, Mode: "none", Source: "none", Provider: "none"}, false, nil
	}
}

func loadLegacyAuthStatus(path string, payload []byte) (Status, bool, error) {
	var cred FileCredential
	if err := json.Unmarshal(payload, &cred); err != nil {
		return Status{}, false, err
	}
	if cred.Provider == "" && cred.APIKey == "" && cred.SavedAt == "" {
		return Status{}, false, nil
	}
	if cred.APIKey == "" {
		return Status{Authenticated: false, Mode: "none", Source: "none", Provider: "none"}, true, nil
	}
	provider := cred.Provider
	if provider == "" {
		provider = "openai"
	}
	return Status{
		Authenticated: true,
		Mode:          "api_key",
		Source:        path,
		Provider:      provider,
	}, true, nil
}

func codexHome(homeDir string) string {
	if homeDir != "" {
		return homeDir
	}
	if value := os.Getenv("CODEX_HOME"); value != "" {
		return value
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return ".codex"
	}
	return filepath.Join(userHome, ".codex")
}

func hasStructuredTokens(payload json.RawMessage) bool {
	if len(payload) == 0 || string(payload) == "null" {
		return false
	}
	var anyValue any
	if err := json.Unmarshal(payload, &anyValue); err != nil {
		return false
	}
	switch typed := anyValue.(type) {
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return anyValue != nil
	}
}
