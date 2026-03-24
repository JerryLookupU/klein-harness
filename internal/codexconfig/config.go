package codexconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Profile struct {
	Model          string `json:"model"`
	ApprovalPolicy string `json:"approvalPolicy"`
	SandboxMode    string `json:"sandboxMode"`
}

type Config struct {
	Model          string             `json:"model"`
	ApprovalPolicy string             `json:"approvalPolicy"`
	SandboxMode    string             `json:"sandboxMode"`
	Profiles       map[string]Profile `json:"profiles"`
}

func Load(homeDir string) (Config, error) {
	path := filepath.Join(resolveCodexHome(homeDir), "config.toml")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Profiles: map[string]Profile{}}, nil
		}
		return Config{}, err
	}
	defer file.Close()

	config := Config{Profiles: map[string]Profile{}}
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		key, value, ok := parseAssignment(line)
		if !ok {
			continue
		}
		switch {
		case section == "":
			assignProfileField(&Profile{
				Model:          config.Model,
				ApprovalPolicy: config.ApprovalPolicy,
				SandboxMode:    config.SandboxMode,
			}, key, value, &config)
		case strings.HasPrefix(section, "profiles."):
			name := strings.Trim(strings.TrimPrefix(section, "profiles."), `"`)
			profile := config.Profiles[name]
			assignProfileField(&profile, key, value, nil)
			config.Profiles[name] = profile
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func Effective(config Config, profileName string, overrides Profile) Profile {
	effective := Profile{
		Model:          config.Model,
		ApprovalPolicy: config.ApprovalPolicy,
		SandboxMode:    config.SandboxMode,
	}
	if profileName != "" {
		if profile, ok := config.Profiles[profileName]; ok {
			if profile.Model != "" {
				effective.Model = profile.Model
			}
			if profile.ApprovalPolicy != "" {
				effective.ApprovalPolicy = profile.ApprovalPolicy
			}
			if profile.SandboxMode != "" {
				effective.SandboxMode = profile.SandboxMode
			}
		}
	}
	if overrides.Model != "" {
		effective.Model = overrides.Model
	}
	if overrides.ApprovalPolicy != "" {
		effective.ApprovalPolicy = overrides.ApprovalPolicy
	}
	if overrides.SandboxMode != "" {
		effective.SandboxMode = overrides.SandboxMode
	}
	if effective.Model == "" {
		effective.Model = "gpt-5.4"
	}
	if effective.ApprovalPolicy == "" {
		effective.ApprovalPolicy = "never"
	}
	if effective.SandboxMode == "" {
		effective.SandboxMode = "workspace-write"
	}
	return effective
}

func parseAssignment(line string) (string, string, bool) {
	index := strings.Index(line, "=")
	if index <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:index])
	value := strings.TrimSpace(line[index+1:])
	value = strings.Trim(value, `"`)
	return key, value, key != ""
}

func assignProfileField(profile *Profile, key, value string, config *Config) {
	switch key {
	case "model":
		if config != nil {
			config.Model = value
			return
		}
		profile.Model = value
	case "approval_policy":
		if config != nil {
			config.ApprovalPolicy = value
			return
		}
		profile.ApprovalPolicy = value
	case "sandbox_mode":
		if config != nil {
			config.SandboxMode = value
			return
		}
		profile.SandboxMode = value
	}
}

func resolveCodexHome(homeDir string) string {
	if homeDir != "" {
		return homeDir
	}
	if value := os.Getenv("CODEX_HOME"); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".codex"
	}
	return filepath.Join(home, ".codex")
}
