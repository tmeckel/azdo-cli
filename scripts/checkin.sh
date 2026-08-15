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

    # Only retry when a pre-commit hook actually modified staged files
    # (e.g. end-of-file fixer). A failing hook that changed nothing is a
    # fatal error (e.g. yamllint): bail out instantly, never retry.
    local modified
    modified=$(git diff --name-only | grep -Fxf <(printf '%s\n' "$staged_before") || true)
    if [[ -z "$modified" ]]; then
      echo "Error: pre-commit hook failed without modifying any staged file; aborting check-in." >&2
      echo "       Fix the failing hook and re-run." >&2
      return 1
    fi
    if typos_hook_modified_file "$modified"; then
      echo "Error: typos pre-commit hook modified staged files; aborting check-in, no retry." >&2
      echo "       Review the typo fixes, then re-run the check-in." >&2
      return 1
    fi

    attempts=$((attempts + 1))
    if [[ $attempts -ge 5 ]]; then
      echo "Error: hooks modified files but the commit still failed $attempts times; giving up. Inspect the working tree and re-run." >&2
      return 1
    fi
    echo "> pre-commit hook modified files; re-staging and retrying:"
    echo "$modified" | sed 's/^/    /'
    # shellcheck disable=SC2086
    git add -- $modified
  done
}

# Detect whether the typos pre-commit hook rewrote any of the given files.
# A staged file whose working-tree version equals the staged version after
# running typos in fix mode was rewritten by the hook. Typo fixes must be
# reviewed deliberately, so callers abort instead of re-staging and retrying.
typos_hook_modified_file() {
  command -v typos >/dev/null 2>&1 || return 1
  local cfg=()
  if [ -f .typos.toml ]; then
    cfg=(-c "$(pwd)/.typos.toml")
  fi
  local f staged_copy
  while IFS= read -r f; do
    [[ -n "$f" ]] || continue
    staged_copy=$(mktemp)
    if git show ":$f" > "$staged_copy" 2>/dev/null; then
      typos --write-changes "${cfg[@]}" "$staged_copy" >/dev/null 2>&1 || true
      if cmp -s "$staged_copy" "$f"; then
        rm -f "$staged_copy"
        return 0
      fi
    fi
    rm -f "$staged_copy"
  done <<< "$1"
  return 1
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
  { git diff --name-only; git ls-files --others --exclude-standard; } | grep -v 'checkin.sh' | sort -u
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
