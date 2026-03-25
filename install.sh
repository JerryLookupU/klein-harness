#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_DIR="${SCRIPT_DIR}/skills"
CODEX_BASE="${CODEX_HOME:-$HOME/.codex}"
DEST_DIR="${CODEX_BASE}/skills"
BIN_DIR="${CODEX_BASE}/bin"
GLOBAL_AGENTS_FILE=""
CONFIG_FILE=""
FORCE=0
UPDATE_SHELL_RC=1
PATH_RC_ACTION="unchanged"
HELPERS_INSTALLED=0
HELPER_NAMES=()
HARNESS_INSTALLED=0
MANAGED_GLOBAL_AGENTS_START="<!-- klein-harness managed global instructions:start -->"
MANAGED_GLOBAL_AGENTS_END="<!-- klein-harness managed global instructions:end -->"
MANAGED_PROFILES_START="# >>> klein-harness managed codex profiles >>>"
MANAGED_PROFILES_END="# <<< klein-harness managed codex profiles <<<"

shopt -s nullglob

if ! command -v tmux >/dev/null 2>&1; then
  echo "tmux is not installed. Install can continue, but bootstrap/daemon features will be limited." >&2
  if [[ "$(uname)" == "Darwin" ]]; then
    echo "macOS: brew install tmux" >&2
  else
    echo "Ubuntu/Debian: sudo apt-get install tmux" >&2
    echo "Fedora/RHEL: sudo dnf install tmux" >&2
  fi
fi

resolve_bin_dir_from_dest() {
  local dest="$1"
  local dest_parent
  local dest_leaf

  dest_parent="$(dirname "$dest")"
  dest_leaf="$(basename "$dest")"
  if [[ "$dest_leaf" == "skills" ]]; then
    printf '%s/bin\n' "$dest_parent"
  else
    printf '%s\n' "$BIN_DIR"
  fi
}

detect_shell_rc() {
  local shell_name="${SHELL##*/}"

  case "$shell_name" in
    zsh)
      printf '%s\n' "${ZDOTDIR:-$HOME}/.zshrc"
      ;;
    bash)
      if [[ -f "$HOME/.bashrc" || ! -f "$HOME/.bash_profile" ]]; then
        printf '%s\n' "$HOME/.bashrc"
      else
        printf '%s\n' "$HOME/.bash_profile"
      fi
      ;;
    fish)
      printf '%s\n' "$HOME/.config/fish/config.fish"
      ;;
    *)
      printf '%s\n' "$HOME/.profile"
      ;;
  esac
}

ensure_bin_dir_on_path() {
  local rc_file="$1"
  local path_line=""

  if [[ ":$PATH:" == *":$BIN_DIR:"* ]]; then
    PATH_RC_ACTION="shell-ready"
    return 0
  fi

  mkdir -p "$(dirname "$rc_file")"
  touch "$rc_file"

  case "$rc_file" in
    *.fish)
      path_line="fish_add_path \"$BIN_DIR\""
      ;;
    *)
      path_line="export PATH=\"$BIN_DIR:\$PATH\""
      ;;
  esac

  if grep -Fqs "$path_line" "$rc_file"; then
    PATH_RC_ACTION="rc-already-configured"
    return 0
  fi

  {
    printf '\n'
    printf '# Added by klein-harness install.sh\n'
    printf '%s\n' "$path_line"
  } >> "$rc_file"
  PATH_RC_ACTION="rc-updated"
}

install_helper_scripts() {
  local helper_src=""
  local helper_name=""
  local helper_dst=""

  for helper_src in "$SCRIPT_DIR"/scripts/*.sh; do
    helper_name="$(basename "$helper_src" .sh)"
    helper_dst="${BIN_DIR}/${helper_name}"
    install -m 755 "$helper_src" "$helper_dst"
    HELPERS_INSTALLED=1
    HELPER_NAMES+=("$helper_name")
  done
}

install_harness_binary() {
  if ! command -v go >/dev/null 2>&1; then
    echo "Go is not installed; skipping harness binary build. Shell wrappers can still fall back to 'go run' from the repo checkout." >&2
    return 0
  fi
  mkdir -p "$BIN_DIR"
  (
    cd "$SCRIPT_DIR"
    go build -o "$BIN_DIR/harness" ./cmd/harness
  )
  HARNESS_INSTALLED=1
}

managed_global_agents_content() {
  cat <<EOF
${MANAGED_GLOBAL_AGENTS_START}
## Klein-Harness Managed Global Preferences

- Prefer \`jq\` for JSON work and \`yq\` for YAML work when those tools are available.
- Prefer \`fd\` for file discovery, \`tree\` for directory overviews, \`delta\` for diff review, and \`rg\` for non-trivial text or code search.
- Check whether a preferred tool exists before you suggest or invoke it. If it is unavailable, say so and choose the closest verified fallback.
- Do not guess URLs, filenames, command syntax, flags, API shapes, or config keys. Verify them from repo facts, \`--help\`, or the relevant documentation first.
- When uncertain, prefer reading documentation or repository evidence over inference.

${MANAGED_GLOBAL_AGENTS_END}
EOF
}

managed_profiles_content() {
  cat <<EOF
${MANAGED_PROFILES_START}
# Managed by klein-harness install.sh.
# These profile names are updated in place on re-install.

[profiles."klein-orchestrator"]
model = "gpt-5.4"
approval_policy = "never"
sandbox_mode = "workspace-write"

[profiles."klein-worker"]
model = "gpt-5.3-codex"
approval_policy = "never"
sandbox_mode = "workspace-write"

[profiles."klein-research"]
model = "gpt-5.4"
approval_policy = "on-request"
sandbox_mode = "read-only"
${MANAGED_PROFILES_END}
EOF
}

upsert_managed_block() {
  local path="$1"
  local start_marker="$2"
  local end_marker="$3"
  local content="$4"
  local label="$5"
  local has_start=0
  local has_end=0
  local tmp_path="${path}.tmp"
  local content_tmp=""

  mkdir -p "$(dirname "$path")"

  if [[ ! -f "$path" ]]; then
    printf '%s\n' "$content" > "$path"
    echo "Created ${label}:"
    echo "  - ${path}"
    return 0
  fi

  if grep -Fqs "$start_marker" "$path"; then
    has_start=1
  fi
  if grep -Fqs "$end_marker" "$path"; then
    has_end=1
  fi

  if [[ "$has_start" -eq 1 && "$has_end" -eq 1 ]]; then
    content_tmp="$(mktemp)"
    printf '%s\n' "$content" > "$content_tmp"
    awk -v start="$start_marker" -v end="$end_marker" -v content_path="$content_tmp" '
      function print_content(    line) {
        while ((getline line < content_path) > 0) {
          print line
        }
        close(content_path)
      }
      $0 == start { print_content(); skip=1; next }
      $0 == end { skip=0; next }
      !skip { print }
    ' "$path" > "$tmp_path"
    rm -f "$content_tmp"
    mv "$tmp_path" "$path"
    echo "Updated ${label}:"
    echo "  - ${path}"
    return 0
  fi

  if [[ "$has_start" -eq 1 || "$has_end" -eq 1 ]]; then
    echo "Warning: partial managed block detected in ${path}; appending a fresh managed block." >&2
  fi

  if [[ -s "$path" ]]; then
    printf '\n' >> "$path"
  fi
  printf '%s\n' "$content" >> "$path"
  echo "Appended ${label}:"
  echo "  - ${path}"
}

install_managed_global_instructions() {
  local content
  content="$(managed_global_agents_content)"
  upsert_managed_block \
    "$GLOBAL_AGENTS_FILE" \
    "$MANAGED_GLOBAL_AGENTS_START" \
    "$MANAGED_GLOBAL_AGENTS_END" \
    "$content" \
    "managed global AGENTS block"
}

install_managed_codex_profiles() {
  local content
  content="$(managed_profiles_content)"
  upsert_managed_block \
    "$CONFIG_FILE" \
    "$MANAGED_PROFILES_START" \
    "$MANAGED_PROFILES_END" \
    "$content" \
    "managed Codex profiles"
}

usage() {
  cat <<'EOF'
Usage:
  ./install.sh [skill_name ...] [--dest <path>] [--bin-dir <path>] [--force] [--no-shell-rc]

Examples:
  ./install.sh
  ./install.sh --force
  ./install.sh --dest ~/.codex/skills --force

Notes:
  - No skill name means install all skills under ./skills/.
  - Default destination is $CODEX_HOME/skills (or ~/.codex/skills if CODEX_HOME is not set).
  - The canonical `harness` CLI and compatibility wrappers are installed into the matching bin directory (default: $CODEX_HOME/bin) when Go is available.
  - Managed global preferences are written to $CODEX_HOME/AGENTS.md.
  - Managed Codex profiles are written to $CODEX_HOME/config.toml.
EOF
}

skills=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    -f|--force)
      FORCE=1
      shift
      ;;
    -d|--dest)
      if [[ $# -lt 2 ]]; then
        echo "Error: --dest requires a path." >&2
        exit 1
      fi
      DEST_DIR="$2"
      BIN_DIR="$(resolve_bin_dir_from_dest "$DEST_DIR")"
      shift 2
      ;;
    --bin-dir)
      if [[ $# -lt 2 ]]; then
        echo "Error: --bin-dir requires a path." >&2
        exit 1
      fi
      BIN_DIR="$2"
      shift 2
      ;;
    --no-shell-rc)
      UPDATE_SHELL_RC=0
      shift
      ;;
    -*)
      echo "Error: unknown option '$1'." >&2
      usage
      exit 1
      ;;
    *)
      skills+=("$1")
      shift
      ;;
  esac
done

DEST_DIR="${DEST_DIR/#\~/$HOME}"
BIN_DIR="${BIN_DIR/#\~/$HOME}"
if [[ "$(basename "$DEST_DIR")" == "skills" ]]; then
  CODEX_BASE="$(dirname "$DEST_DIR")"
fi
CODEX_BASE="${CODEX_BASE/#\~/$HOME}"
GLOBAL_AGENTS_FILE="${CODEX_BASE}/AGENTS.md"
CONFIG_FILE="${CODEX_BASE}/config.toml"

if [[ ! -d "$SKILLS_DIR" ]]; then
  echo "Error: skills directory not found: $SKILLS_DIR" >&2
  exit 1
fi

available_skills=()
for entry in "$SKILLS_DIR"/*; do
  if [[ -d "$entry" && -f "$entry/SKILL.md" ]]; then
    available_skills+=("$(basename "$entry")")
  fi
done

if [[ ${#available_skills[@]} -eq 0 ]]; then
  echo "Error: no installable skills found in $SKILLS_DIR" >&2
  exit 1
fi

if [[ ${#skills[@]} -eq 0 ]]; then
  skills=("${available_skills[@]}")
fi

mkdir -p "$DEST_DIR"
mkdir -p "$BIN_DIR"

installed=()
skipped=()
existing=()
requested_missing=()

for skill in "${skills[@]}"; do
  src="${SKILLS_DIR}/${skill}"
  dst="${DEST_DIR}/${skill}"

  if [[ ! -f "${src}/SKILL.md" ]]; then
    echo "Skip '${skill}': skill not found at ${src}" >&2
    requested_missing+=("$skill")
    continue
  fi

  if [[ -e "$dst" ]]; then
    if [[ "$FORCE" -eq 1 ]]; then
      rm -rf "$dst"
    else
      echo "Keep '${skill}': ${dst} already exists" >&2
      existing+=("$skill")
      continue
    fi
  fi

  cp -R "$src" "$dst"
  installed+=("$skill")
done

echo "Destination: $DEST_DIR"

if [[ ${#installed[@]} -gt 0 ]]; then
  echo "Installed skills:"
  for skill in "${installed[@]}"; do
    echo "  - $skill"
  done
fi

if [[ ${#existing[@]} -gt 0 ]]; then
  echo "Kept existing skills:"
  for skill in "${existing[@]}"; do
    echo "  - $skill"
  done
fi

if [[ ${#requested_missing[@]} -gt 0 ]]; then
  echo "Skipped missing skills:"
  for skill in "${requested_missing[@]}"; do
    echo "  - $skill"
  done
fi

if [[ ${#installed[@]} -eq 0 && ${#existing[@]} -eq 0 ]]; then
  echo "No skills were installed."
  exit 1
fi

install_managed_global_instructions
install_managed_codex_profiles

if printf '%s\n%s\n' "${installed[*]:-}" "${existing[*]:-}" | tr ' ' '\n' | grep -qx 'klein-harness'; then
  install_harness_binary
  install_helper_scripts

  if [[ "$UPDATE_SHELL_RC" -eq 1 ]]; then
    SHELL_RC="$(detect_shell_rc)"
    ensure_bin_dir_on_path "$SHELL_RC"

    case "$PATH_RC_ACTION" in
      shell-ready)
        echo "Helper command is already available in PATH."
        ;;
      rc-already-configured)
        echo "Shell config already contains ${BIN_DIR}."
        echo "Open a new shell or run:"
        echo "  - source ${SHELL_RC}"
        echo "You can also run the primary command directly:"
        echo "  - ${BIN_DIR}/harness-submit"
        ;;
      rc-updated)
        echo "Updated shell config:"
        echo "  - ${SHELL_RC}"
        echo "Open a new shell or run:"
        echo "  - source ${SHELL_RC}"
        echo "You can also run the primary command directly:"
        echo "  - ${BIN_DIR}/harness-submit"
        ;;
      *)
        echo "Updated shell config:"
        echo "  - ${SHELL_RC}"
        echo "Open a new shell or run:"
        echo "  - source ${SHELL_RC}"
        echo "You can also run the primary command directly:"
        echo "  - ${BIN_DIR}/harness-submit"
        ;;
    esac
  else
    echo "PATH update skipped (--no-shell-rc)."
    echo "Run the helper directly or add this to your shell config:"
    echo "  - export PATH=\"${BIN_DIR}:\$PATH\""
  fi
fi

if [[ ${#installed[@]} -eq 0 && "$HELPERS_INSTALLED" -eq 0 ]]; then
  echo "Nothing changed."
fi

if [[ "$HELPERS_INSTALLED" -eq 1 ]]; then
  if [[ "$HARNESS_INSTALLED" -eq 1 ]]; then
    echo "Canonical CLI:"
    echo "  - harness"
  fi
  echo "Compatibility wrappers:"
  echo "  - harness-submit"
  echo "  - harness-tasks"
  echo "  - harness-task"
  echo "  - harness-control"
  echo "Compatibility wrappers remain installed, but the canonical runtime entrypoint is the Go `harness` CLI."
fi

echo "Managed verification hints:"
echo "  - rg -n 'klein-harness managed' \"${GLOBAL_AGENTS_FILE}\""
echo "  - rg -n 'klein-harness managed|profiles\\.\"klein-(orchestrator|worker|research)\"' \"${CONFIG_FILE}\""
echo "Skill verification hints:"
for skill in markdown-fetch generate-contributor-guide; do
  if [[ -f "${DEST_DIR}/${skill}/SKILL.md" ]]; then
    echo "  - test -f \"${DEST_DIR}/${skill}/SKILL.md\""
  fi
done
if [[ -f "${DEST_DIR}/generate-contributor-guide/references/analysis-checklist.md" ]]; then
  echo "  - test -f \"${DEST_DIR}/generate-contributor-guide/references/analysis-checklist.md\""
fi
echo "Restart Codex to pick up new skills."
