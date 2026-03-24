package codexconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndEffectiveProfile(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(`
model = "gpt-5.4"
approval_policy = "never"

[profiles.safe]
model = "gpt-5.4-mini"
sandbox_mode = "read-only"
approval_policy = "on-request"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := Load(home)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if config.Model != "gpt-5.4" {
		t.Fatalf("unexpected model: %+v", config)
	}

	effective := Effective(config, "safe", Profile{})
	if effective.Model != "gpt-5.4-mini" || effective.SandboxMode != "read-only" || effective.ApprovalPolicy != "on-request" {
		t.Fatalf("unexpected effective profile: %+v", effective)
	}
}

func TestEffectiveDefaultsToGpt54(t *testing.T) {
	effective := Effective(Config{}, "", Profile{})
	if effective.Model != "gpt-5.4" || effective.ApprovalPolicy != "never" || effective.SandboxMode != "workspace-write" {
		t.Fatalf("unexpected defaults: %+v", effective)
	}
}
