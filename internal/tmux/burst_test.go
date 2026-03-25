package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"klein-harness/internal/dispatch"
)

func TestRunBoundedBurstWritesOutcome(t *testing.T) {
	fakeBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	tmuxRoot := filepath.Join(t.TempDir(), "tmux")
	if err := os.MkdirAll(tmuxRoot, 0o755); err != nil {
		t.Fatalf("mkdir fake tmux root: %v", err)
	}
	writeExecutable(t, filepath.Join(fakeBin, "tmux"), fakeTmuxForBurstTest)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_TMUX_ROOT", tmuxRoot)

	root := t.TempDir()
	checkpointPath := filepath.Join(root, "checkpoints", "task.json")
	outcomePath := filepath.Join(root, "checkpoints", "outcome.json")
	promptPath := filepath.Join(root, "runner-prompt.md")
	if err := os.WriteFile(promptPath, []byte("manifest-path\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	capturedPath := filepath.Join(root, "captured.txt")
	result, err := RunBoundedBurst(BurstRequest{
		Root:           root,
		TaskID:         "T-1",
		DispatchID:     "dispatch-1",
		WorkerID:       "worker-1",
		Cwd:            root,
		Command:        "cat > captured.txt",
		PromptPath:     promptPath,
		CheckpointPath: checkpointPath,
		OutcomePath:    outcomePath,
		Artifacts:      []string{"artifact-a"},
		Budget: dispatch.Budget{
			MaxMinutes: 1,
		},
	})
	if err != nil {
		t.Fatalf("run bounded burst: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("expected succeeded status, got %s", result.Status)
	}
	captured, err := os.ReadFile(capturedPath)
	if err != nil {
		t.Fatalf("read captured stdin: %v", err)
	}
	if string(captured) != "manifest-path\n" {
		t.Fatalf("unexpected captured stdin: %q", string(captured))
	}
	if !slicesContain(result.Artifacts, "artifact-a") {
		t.Fatalf("expected artifact list to include manifest artifacts: %#v", result.Artifacts)
	}
	if !slicesContain(result.Artifacts, checkpointPath) || !slicesContain(result.Artifacts, outcomePath) {
		t.Fatalf("expected checkpoint/outcome artifacts: %#v", result.Artifacts)
	}
	if result.SessionName == "" || result.LogPath == "" {
		t.Fatalf("expected tmux metadata in result: %#v", result)
	}
	if !strings.HasPrefix(result.SessionName, "kh_") {
		t.Fatalf("expected prefixed tmux session name, got %q", result.SessionName)
	}
	if !strings.Contains(result.SessionName, "_T-1_dispatch-1") {
		t.Fatalf("expected scoped tmux session name to retain task/dispatch, got %q", result.SessionName)
	}
}

func TestDefaultSessionNameIncludesScopeToken(t *testing.T) {
	nameA := defaultSessionName("/tmp/repo-a", "", "T-1", "dispatch-1")
	nameB := defaultSessionName("/tmp/repo-b", "", "T-1", "dispatch-1")
	if nameA == nameB {
		t.Fatalf("expected different session names for different roots, got %q", nameA)
	}
	if !strings.Contains(nameA, "_T-1_dispatch-1") || !strings.Contains(nameB, "_T-1_dispatch-1") {
		t.Fatalf("expected names to retain task and dispatch identifiers: %q %q", nameA, nameB)
	}
}

func slicesContain(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

const fakeTmuxForBurstTest = `#!/usr/bin/env bash
set -euo pipefail
ROOT="${FAKE_TMUX_ROOT:?missing FAKE_TMUX_ROOT}"
mkdir -p "$ROOT"
cmd="${1:-}"
shift || true
session_dir() {
  printf '%s/%s' "$ROOT" "$1"
}
case "$cmd" in
  new-session)
    session=""
    cwd=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        -d) shift ;;
        -s) session="$2"; shift 2 ;;
        -c) cwd="$2"; shift 2 ;;
        *) shift ;;
      esac
    done
    dir="$(session_dir "$session")"
    mkdir -p "$dir"
    printf '%s' "$cwd" > "$dir/cwd"
    : > "$dir/pane"
    ;;
  set-option)
    ;;
  pipe-pane)
    session=""
    pipe_cmd=""
    while [[ $# -gt 0 ]]; do
      case "$1" in
        -o) shift ;;
        -t) session="$2"; shift 2 ;;
        *) pipe_cmd="$1"; shift ;;
      esac
    done
    dir="$(session_dir "$session")"
    log_path="${pipe_cmd#*>> }"
    log_path="${log_path#\'}"
    log_path="${log_path%\'}"
    printf '%s' "$log_path" > "$dir/logpath"
    mkdir -p "$(dirname "$log_path")"
    : > "$log_path"
    ;;
  send-keys)
    session="$2"
    command="$3"
    dir="$(session_dir "$session")"
    cwd="$(cat "$dir/cwd")"
    log_path="$(cat "$dir/logpath")"
    output_file="$dir/output"
    set +e
    (
      cd "$cwd"
      /bin/sh -c "$command"
    ) >"$output_file" 2>&1
    code=$?
    set -e
    cp "$output_file" "$dir/pane"
    cat "$output_file" >> "$log_path"
    printf '%s' "$code" > "$dir/last-exit"
    ;;
  capture-pane)
    session="$3"
    cat "$(session_dir "$session")/pane"
    ;;
  has-session)
    session="$2"
    test -d "$(session_dir "$session")"
    ;;
  kill-session)
    session="$2"
    rm -rf "$(session_dir "$session")"
    ;;
  attach-session)
    session="$2"
    printf 'attached:%s\n' "$session"
    ;;
  *)
    echo "unsupported fake tmux command: $cmd" >&2
    exit 1
    ;;
esac
`
