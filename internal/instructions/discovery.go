package instructions

import (
	"os"
	"path/filepath"
)

type File struct {
	Path   string `json:"path"`
	Scope  string `json:"scope"`
	Exists bool   `json:"exists"`
}

func Discover(startDir, codexHome string) ([]File, error) {
	start, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	files := make([]File, 0, 8)
	for _, name := range []string{"AGENTS.override.md", "AGENTS.md"} {
		path := filepath.Join(resolveCodexHome(codexHome), name)
		if exists(path) {
			files = append(files, File{Path: path, Scope: "global", Exists: true})
		}
	}
	root := repoRoot(start)
	chain := dirChain(root, start)
	for _, dir := range chain {
		for _, name := range []string{"AGENTS.override.md", "AGENTS.md"} {
			path := filepath.Join(dir, name)
			if exists(path) {
				scope := "project"
				if dir != root {
					scope = "nested"
				}
				files = append(files, File{Path: path, Scope: scope, Exists: true})
			}
		}
	}
	return files, nil
}

func resolveCodexHome(codexHome string) string {
	if codexHome != "" {
		return codexHome
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

func repoRoot(start string) string {
	current := start
	last := ""
	for current != last {
		if exists(filepath.Join(current, ".git")) {
			return current
		}
		last = current
		current = filepath.Dir(current)
	}
	return start
}

func dirChain(root, current string) []string {
	if root == current {
		return []string{root}
	}
	dirs := []string{}
	for dir := current; ; dir = filepath.Dir(dir) {
		dirs = append([]string{dir}, dirs...)
		if dir == root {
			return dirs
		}
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
