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
  local body=${2:-}
  if ! has_staged_changes; then
    echo "(skip) no staged changes for commit: $msg"
    return 0
  fi

  # Files staged for this commit, captured before the first attempt. Used to
  # detect files modified by the pre-commit hook afterwards.
  local staged_before
  staged_before=$(git diff --cached --name-only | sort -u)

  local attempts=0
  while true; do
    if [[ -n "$body" ]]; then
      echo "> git commit -m \"$msg\" -m \"$body\""
      if git commit -m "$msg" -m "$body"; then
        return 0
      fi
    else
      echo "> git commit -m \"$msg\""
      if git commit -m "$msg"; then
        return 0
      fi
    fi

    # The pre-commit hook may have modified files (e.g. an end-of-file
    # fixer). Re-stage exactly the files it touched and retry the commit.
    attempts=$((attempts + 1))
    if [[ $attempts -ge 5 ]]; then
      echo "Error: commit failed $attempts times; giving up. Inspect the working tree and re-run." >&2
      return 1
    fi
    local modified
    modified=$(git diff --name-only | grep -Fxf <(printf '%s\n' "$staged_before") || true)
    if [[ -z "$modified" ]]; then
      echo "Error: commit failed and no staged files were modified by a hook; fix manually." >&2
      return 1
    fi
    echo "> pre-commit hook modified files; re-staging and retrying:"
    echo "$modified" | sed 's/^/    /'
    # shellcheck disable=SC2086
    git add -- $modified
  done
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

changed_files() {
  { git diff --name-only; git ls-files --others --exclude-standard; } | sort -u
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

if changed_files | grep -q .; then
  echo "Error: leftover uncommitted changes:" >&2
  changed_files | sed 's/^/  /' >&2
  exit 1
fi

branch=$(git branch --show-current)
echo "==> Done. Working tree is clean."
echo "    Next: git push -u origin ${branch}"
echo "    PR:   gh pr create --title 'feat(boards): add work-item create command' --body 'Fixes #203'"
