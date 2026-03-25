//go:build integration

package runtime_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"klein-harness/internal/adapter"
	"klein-harness/internal/query"
	"klein-harness/internal/runtime"
	"klein-harness/internal/state"
	"klein-harness/internal/verify"
)

func TestIntegrationInitSubmitRunOnceCompletes(t *testing.T) {
	env := newHarnessEnv(t)
	writeFile(t, filepath.Join(env.root, "README.md"), "# demo\n")

	runHarness(t, env, "init", env.root)
	submit := runHarness(t, env, "submit", env.root, "--goal", "Fix failing runtime bug")
	if submit.Task.TaskID != "T-001" {
		t.Fatalf("expected first task id, got %q", submit.Task.TaskID)
	}
	result := runHarnessDaemon(t, env)
	if result.RuntimeStatus != "completed" {
		t.Fatalf("expected completed runtime, got %#v", result)
	}
	task, err := adapter.LoadTask(env.root, "T-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != "completed" {
		t.Fatalf("expected completed task, got %#v", task)
	}
	if task.TmuxSession == "" || task.TmuxLogPath == "" {
		t.Fatalf("expected tmux metadata on task: %#v", task)
	}
	if _, err := os.Stat(filepath.Join(env.root, ".harness", "checkpoints", "T-001", "outcome.json")); err != nil {
		t.Fatalf("expected outcome artifact: %v", err)
	}
	var verification struct {
		Tasks map[string]struct {
			Status    string `json:"status"`
			Completed bool   `json:"completed"`
		} `json:"tasks"`
	}
	if err := state.LoadJSON(filepath.Join(env.root, ".harness", "state", "verification-summary.json"), &verification); err != nil {
		t.Fatalf("load verification summary: %v", err)
	}
	if verification.Tasks["T-001"].Status != "passed" || !verification.Tasks["T-001"].Completed {
		t.Fatalf("unexpected verification summary: %#v", verification.Tasks["T-001"])
	}
}

func TestIntegrationBurstFailureEmitsReplan(t *testing.T) {
	env := newHarnessEnv(t)
	t.Setenv("FAKE_CODEX_MODE", "fail")

	runHarness(t, env, "submit", env.root, "--goal", "Fix bug that still fails")
	result := runHarnessDaemon(t, env)
	if result.RuntimeStatus != "needs_replan" || result.FollowUpEvent != "replan.emitted" {
		t.Fatalf("expected replan result, got %#v", result)
	}
	task, err := adapter.LoadTask(env.root, "T-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != "needs_replan" {
		t.Fatalf("expected task to need replan, got %#v", task)
	}
}

func TestIntegrationResumeSessionFlow(t *testing.T) {
	env := newHarnessEnv(t)
	t.Setenv("FAKE_CODEX_SESSION_ID", "sess-first")

	runHarness(t, env, "submit", env.root, "--goal", "Continue the previous task and resume work")
	first := runHarnessDaemon(t, env)
	if first.RuntimeStatus != "completed" {
		t.Fatalf("expected first run to complete, got %#v", first)
	}
	if _, err := runtime.RestartFromStage(env.root, "T-001", "queued", "resume test"); err != nil {
		t.Fatalf("restart from stage: %v", err)
	}
	t.Setenv("FAKE_CODEX_LOG", filepath.Join(env.tmp, "fake-codex.log"))
	second := runHarnessDaemon(t, env)
	if second.RuntimeStatus != "completed" {
		t.Fatalf("expected resumed run to complete, got %#v", second)
	}
	payload, err := os.ReadFile(filepath.Join(env.tmp, "fake-codex.log"))
	if err != nil {
		t.Fatalf("read fake codex log: %v", err)
	}
	if !strings.Contains(string(payload), "exec resume sess-first") {
		t.Fatalf("expected resume command, got %q", string(payload))
	}
}

func TestIntegrationControlStatusAttachAndArchive(t *testing.T) {
	env := newHarnessEnv(t)

	runHarness(t, env, "submit", env.root, "--goal", "Implement a recommendation flow")
	result := runHarnessDaemon(t, env)
	if result.RuntimeStatus != "completed" {
		t.Fatalf("expected completed runtime, got %#v", result)
	}
	status := runTaskView(t, env, "control", env.root, "task", "T-001", "status")
	if status.AttachCommand == "" || status.Task.TmuxSession == "" {
		t.Fatalf("expected attach metadata, got %#v", status)
	}
	attach := runJSONMap(t, env, env.harnessBin, "control", env.root, "task", "T-001", "attach")
	if attach["attachCommand"] == "" {
		t.Fatalf("expected attach command in output: %#v", attach)
	}
	archived := runTaskJSON(t, env, "control", env.root, "task", "T-001", "archive")
	if archived.Status != "archived" {
		t.Fatalf("expected archived task, got %#v", archived)
	}
	var gate verify.CompletionGate
	if err := state.LoadJSON(filepath.Join(env.root, ".harness", "state", "completion-gate.json"), &gate); err != nil {
		t.Fatalf("load gate: %v", err)
	}
	if !gate.Retired {
		t.Fatalf("expected retired gate after archive: %#v", gate)
	}
}

func TestIntegrationOwnedPathsViolationTriggersReplan(t *testing.T) {
	env := newHarnessEnv(t)
	t.Setenv("FAKE_CODEX_CHANGED_PATH", "outside.txt")

	runHarness(t, env, "submit", env.root, "--goal", "Fix risky bug")
	task, err := adapter.LoadTask(env.root, "T-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	task.OwnedPaths = []string{"allowed/**"}
	if err := adapter.UpsertTask(env.root, task); err != nil {
		t.Fatalf("update task: %v", err)
	}
	result := runHarnessDaemon(t, env)
	if result.RuntimeStatus != "needs_replan" || result.BurstStatus != "failed" {
		t.Fatalf("expected ownedPaths failure result, got %#v", result)
	}
}

func TestIntegrationLegacyWrappersForwardToGoCLI(t *testing.T) {
	env := newHarnessEnv(t)

	runScript(t, env, "scripts/harness-submit.sh", env.root, "--goal", "Resume work through wrapper")
	task, err := adapter.LoadTask(env.root, "T-001")
	if err != nil {
		t.Fatalf("load task after wrapper submit: %v", err)
	}
	if task.Status != "queued" {
		t.Fatalf("expected queued task after wrapper submit, got %#v", task)
	}
	runScript(t, env, "scripts/harness-tasks.sh", env.root)
	runScript(t, env, "scripts/harness-control.sh", env.root, "task", "T-001", "restart-from-stage", "queued")
	restarted, err := adapter.LoadTask(env.root, "T-001")
	if err != nil {
		t.Fatalf("load restarted task: %v", err)
	}
	if restarted.Status != "queued" {
		t.Fatalf("expected queued task after wrapper control, got %#v", restarted)
	}
}

type harnessEnv struct {
	repoRoot   string
	root       string
	tmp        string
	harnessBin string
}

type submitResult struct {
	Task struct {
		TaskID string `json:"taskId"`
	} `json:"task"`
}

func newHarnessEnv(t *testing.T) harnessEnv {
	t.Helper()
	repoRoot := repoRoot(t)
	tmp := t.TempDir()
	root := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	fakeBin := filepath.Join(tmp, "fake-bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	fakeTmuxRoot := filepath.Join(tmp, "fake-tmux")
	if err := os.MkdirAll(fakeTmuxRoot, 0o755); err != nil {
		t.Fatalf("mkdir fake tmux root: %v", err)
	}
	writeExecutable(t, filepath.Join(fakeBin, "tmux"), fakeTmuxScript)
	writeExecutable(t, filepath.Join(fakeBin, "codex"), fakeCodexScript)
	harnessBin := filepath.Join(tmp, "harness")
	build := exec.Command("go", "build", "-o", harnessBin, "./cmd/harness")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build harness: %v\n%s", err, output)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_TMUX_ROOT", fakeTmuxRoot)
	t.Setenv("FAKE_CODEX_MODE", "success")
	t.Setenv("FAKE_CODEX_SESSION_ID", "sess-default")
	t.Setenv("FAKE_CODEX_CHANGED_PATH", "README.md")
	t.Setenv("HARNESS_BIN", harnessBin)
	return harnessEnv{
		repoRoot:   repoRoot,
		root:       root,
		tmp:        tmp,
		harnessBin: harnessBin,
	}
}

func runHarness(t *testing.T, env harnessEnv, args ...string) submitResult {
	t.Helper()
	command := exec.Command(env.harnessBin, args...)
	command.Dir = env.repoRoot
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run harness %v: %v\n%s", args, err, output)
	}
	var decoded submitResult
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode harness output: %v\n%s", err, output)
	}
	return decoded
}

func runHarnessDaemon(t *testing.T, env harnessEnv) runtime.RunResult {
	t.Helper()
	command := exec.Command(env.harnessBin, "daemon", "run-once", env.root, "--skip-git-repo-check")
	command.Dir = env.repoRoot
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run daemon: %v\n%s", err, output)
	}
	var decoded runtime.RunResult
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode daemon output: %v\n%s", err, output)
	}
	return decoded
}

func runTaskView(t *testing.T, env harnessEnv, args ...string) query.TaskView {
	t.Helper()
	command := exec.Command(env.harnessBin, args...)
	command.Dir = env.repoRoot
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run harness %v: %v\n%s", args, err, output)
	}
	var decoded query.TaskView
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode task view: %v\n%s", err, output)
	}
	return decoded
}

func runJSONMap(t *testing.T, env harnessEnv, bin string, args ...string) map[string]string {
	t.Helper()
	command := exec.Command(bin, args...)
	command.Dir = env.repoRoot
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %v: %v\n%s", bin, args, err, output)
	}
	var decoded map[string]string
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode json map: %v\n%s", err, output)
	}
	return decoded
}

func runTaskJSON(t *testing.T, env harnessEnv, args ...string) adapter.Task {
	t.Helper()
	command := exec.Command(env.harnessBin, args...)
	command.Dir = env.repoRoot
	command.Env = os.Environ()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run harness %v: %v\n%s", args, err, output)
	}
	var decoded adapter.Task
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("decode task json: %v\n%s", err, output)
	}
	return decoded
}

func runScript(t *testing.T, env harnessEnv, script string, args ...string) {
	t.Helper()
	scriptPath := filepath.Join(env.repoRoot, script)
	command := exec.Command("bash", append([]string{scriptPath}, args...)...)
	command.Dir = env.repoRoot
	command.Env = os.Environ()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run script %s %v: %v\n%s", script, args, err, output)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir file dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	writeFile(t, path, contents)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

const fakeTmuxScript = `#!/usr/bin/env bash
set -euo pipefail
ROOT="${FAKE_TMUX_ROOT:?missing FAKE_TMUX_ROOT}"
mkdir -p "$ROOT"
cmd="${1:-}"
shift || true
printf '%s %s\n' "$cmd" "$*" >> "$ROOT/invocations.log"
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
    [[ "${1:-}" == "-t" ]]
    session="$2"
    command="$3"
    dir="$(session_dir "$session")"
    cwd="$(cat "$dir/cwd")"
    log_path=""
    if [[ -f "$dir/logpath" ]]; then
      log_path="$(cat "$dir/logpath")"
    fi
    output_file="$dir/output"
    set +e
    (
      cd "$cwd"
      /bin/sh -c "$command"
    ) >"$output_file" 2>&1
    code=$?
    set -e
    cp "$output_file" "$dir/pane"
    if [[ -n "$log_path" ]]; then
      cat "$output_file" >> "$log_path"
    fi
    printf '%s' "$code" > "$dir/last-exit"
    ;;
  capture-pane)
    [[ "${1:-}" == "-p" ]]
    [[ "${2:-}" == "-t" ]]
    session="$3"
    dir="$(session_dir "$session")"
    cat "$dir/pane"
    ;;
  has-session)
    [[ "${1:-}" == "-t" ]]
    session="$2"
    test -d "$(session_dir "$session")"
    ;;
  kill-session)
    [[ "${1:-}" == "-t" ]]
    session="$2"
    rm -rf "$(session_dir "$session")"
    ;;
  attach-session)
    [[ "${1:-}" == "-t" ]]
    session="$2"
    printf 'attached:%s\n' "$session"
    ;;
  *)
    echo "unsupported fake tmux command: $cmd" >&2
    exit 1
    ;;
esac
`

const fakeCodexScript = `#!/usr/bin/env bash
set -euo pipefail
mode="${FAKE_CODEX_MODE:-success}"
session_value="${FAKE_CODEX_SESSION_ID:-sess-default}"
changed_path="${FAKE_CODEX_CHANGED_PATH:-README.md}"
last_message=""
resume_mode="fresh"
resume_session=""
args=("$@")
printf '%s\n' "${args[*]}" >> "${FAKE_CODEX_LOG:-/dev/null}"
if [[ "${1:-}" == "exec" ]]; then
  shift
fi
if [[ "${1:-}" == "resume" ]]; then
  resume_mode="resume"
  resume_session="${2:-}"
  shift 2
fi
while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-last-message)
      last_message="$2"
      shift 2
      ;;
    --json|--full-auto|--skip-git-repo-check|--dangerously-bypass-approvals-and-sandbox)
      shift
      ;;
    --model|--sandbox|--add-dir)
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done
prompt="$(cat)"
worker_result="$(printf '%s\n' "$prompt" | sed -n 's/^- \(.*worker-result\.json\)$/\1/p' | head -n 1)"
verify_result="$(printf '%s\n' "$prompt" | sed -n 's/^- \(.*verify\.json\)$/\1/p' | head -n 1)"
handoff="$(printf '%s\n' "$prompt" | sed -n 's/^- \(.*handoff\.md\)$/\1/p' | head -n 1)"
artifact_dir="$(dirname "$worker_result")"
if [[ -z "$worker_result" || -z "$verify_result" || -z "$handoff" ]]; then
  echo "fake codex could not find artifact paths in prompt" >&2
  exit 1
fi
mkdir -p "$artifact_dir"
cat > "$worker_result" <<EOF
{"sessionId":"$session_value","resumeMode":"$resume_mode","resumeSession":"$resume_session","changedPaths":["$changed_path"]}
EOF
printf 'handoff: %s\n' "$mode" > "$handoff"
if [[ -n "$last_message" ]]; then
  printf 'fake codex last message (%s)\n' "$mode" > "$last_message"
fi
case "$mode" in
  success)
    cat > "$verify_result" <<EOF
{"status":"passed","summary":"verification passed","results":[{"name":"smoke","status":"passed"}]}
EOF
    ;;
  blocked)
    cat > "$verify_result" <<EOF
{"status":"blocked","summary":"waiting on evidence","results":[{"name":"smoke","status":"blocked"}]}
EOF
    ;;
  review-ok)
    cat > "$verify_result" <<EOF
{"status":"passed","summary":"verification passed","results":[{"name":"smoke","status":"passed"}],"reviewEvidence":[{"summary":"looks good"}]}
EOF
    ;;
  no-evidence)
    ;;
  fail)
    exit 17
    ;;
  *)
    echo "unsupported fake codex mode: $mode" >&2
    exit 1
    ;;
esac
`
