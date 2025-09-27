#!/usr/bin/env bash
set -euo pipefail

# Idempotent, fail-fast check-in script generated from @CHECKIN.md
# Runs per-file commits: supporting libs first, then CLI commands, then docs.

echo "==> Starting check-in sequence (idempotent)"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "Error: not inside a git repository" >&2
  exit 1
fi

has_staged_changes() {
  if git diff --cached --quiet --exit-code; then
    return 1 # no staged changes
  else
    return 0 # staged changes present
  fi
}

commit_if_staged() {
  local msg=$1
  if has_staged_changes; then
    echo "> git commit -m \"$msg\""
    git commit -m "$msg"
  else
    echo "(skip) no staged changes for commit: $msg"
  fi
}

is_tracked() {
  git ls-files --error-unmatch -- "$1" >/dev/null 2>&1
}

stage_if_exists() {
  for f in "$@"; do
    if [ -e "$f" ]; then
      echo "> git add $f"
      git add -- "$f"
    else
      echo "(skip) missing file: $f"
    fi
  done
}

remove_if_tracked() {
  local f=$1
  if is_tracked "$f"; then
    echo "> git rm -f -- $f"
    git rm -f -- "$f" >/dev/null 2>&1 || git rm -f --cached -- "$f" >/dev/null 2>&1 || true
  else
    echo "(skip) not tracked: $f"
  fi
}

# ==== CHECKIN START ====

# The commands to stage and commit individual files go between CHECKIN START and CHECKIN END.
#
# These delimiters serve as a clear boundary for automated or manual updates:
# - Always commit **exactly one file per commit** to preserve granular history.
# - Use the existing helper functions above: stage_if_exists <file>; commit_if_staged "<message>"
# - Commit messages must follow the Conventional Commits format (type(scope): description) and
#   include a relevant emoji to quickly convey the change type.
# - This section is the **only** place in the script to add, remove, or change commit commands.
# - Tools or AIs updating the script should replace everything between the delimiters, leaving all
#   other parts of the script (helpers, start/end echos) untouched.
# - Avoid staging directories — enumerate each file explicitly to commit one at a time.

# ==== CHECKIN END ====

echo "==> Done. Consider running: go test ./... && make build && git push -u origin <branch>"
