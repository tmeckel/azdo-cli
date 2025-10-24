## Working with `scripts/checkin.sh` — AI Guidance

This document provides detailed instructions on how to **interpret, use, and update** the `scripts/checkin.sh` script based on the workflow from this session.

### Purpose of `scripts/checkin.sh`
- Automates *idempotent* check‑in (Git commit) of changes.
- Uses helper functions within the script (`stage_if_exists`, `commit_if_staged`, etc.) to safely stage and commit files.
- Enforces meaningful, Conventional Commit messages with emojis for quick categorization.

### Current Session Context
In this session, the script was updated to:
- Remove previous hard‑coded file list.
- Add a **new, fixed file list** covering *all changed or new files under* the repository root.
- Group commits logically in the following strict order:

  1. **Build Scripts:** Scripts that affect the build or development process (e.g., `scripts/generate_mocks.sh`).
  2. **Mocks:** Generated mock files (`internal/mocks/`).
  3. **Core libraries/utilities:** Foundational code (`internal/azdo/`, `internal/config/`, etc.).
  4. **CLI Command Implementations:** The `*.go` files for the commands themselves (`internal/cmd/...`).
  5. **Tests:** The `*_test.go` files for the commands.
  6. **Documentation:** Markdown files, etc. (`docs/`).

- Provide verbose commit messages:
  - Must follow Conventional Commits format: `type(scope): description`
  - Include a relevant emoji matching commit type.
  - Describe *what changed*, *why it matters*, and notable impact.

### How to Update the Script — Step‑by‑Step
When an update is needed, follow these exact steps to avoid ambiguity:

1. **Identify Changes**:
   - Run `git status --porcelain` to list *all* modified or untracked files in the repository.
   - Ensure you capture every relevant file path.

2. **Remove Old Entries**:
   - Delete any prior hard‑coded `stage_if_exists` and `commit_if_staged` lines to prevent duplication or stale paths.

3. **Create New Fixed List**:
   - Write one `stage_if_exists <file>` call followed immediately by `commit_if_staged "<message>"` **for each file**.
   - Do not attempt dynamic iteration — we keep the list explicit to control commit order and message content.

4. **Generate Commit Messages**:
   - Infer `type` from change nature by analyzing the change.
   - Then prefix the commit line with one of the following items:
     * `feat` — new feature or functionality.
     * `fix` — bug fixes.
     * `refactor` — restructuring without behavior change.
     * `test` — tests added or improved.
     * `chore` — maintenance work or data updates.
   - Be *verbose*: Commit messages **must** be multi-line, with a concise subject line and a detailed body explaining the *what*, *why*, and *impact* of the change.

     **Good Example:**
     ```
     feat: ✨ add --source-branch flag to create command

     This commit enhances the `repo create` command by adding a new `--source-branch` flag. This allows users to specify a single branch to include when creating a fork, instead of the default behavior of copying all branches. This change also adds documentation to the code explaining the difference between the ParentRepository body parameter and the SourceRef query parameter in the underlying Azure DevOps REST API, clarifying the purpose of each.
     ```

     **Bad Example:**
     ```
     feat: add source branch flag
     ```
   - Append matching emoji (✨, 🐛, ♻️, ✅, 📦, etc.).

5. **Group Logically**:
   - Maintain the strict grouping order defined above: Build Scripts → Mocks → Core → Commands → Tests → Docs.
   - Order matters for commit clarity and history readability.

6. **Preserve Script Helpers**:
   - Always use the provided `stage_if_exists` and `commit_if_staged` functions instead of re‑implementing staging/committing logic.
   - This ensures idempotency and safe execution.

7. **Final Check**:
   - The script should echo staged/committed actions.
   - Skip missing or untracked files gracefully.
   - At the end, retain the summary echo advising further actions (tests, build, push).

### Key Reminders
- **No dynamic loops**: keep explicit paths in the script for predictable commit order.
- **Always remove stale entries** before adding new ones.
- **Commit messages must be human‑readable and specific**.
- Ensure emoji matches type for visual scanning in Git log.
- Context matters — inspect diffs if unsure about commit type.

### Meaning of CHECKIN START / CHECKIN END Delimiters
- The section between these delimiters in `scripts/checkin.sh` is the **only** place where commit commands (`stage_if_exists` + `commit_if_staged`) should be added or removed.
- All changes to commit commands should happen strictly within these markers, leaving helpers and other script parts untouched.
- The delimiters make it trivial for humans or AIs to locate and replace the commit list without altering unrelated logic.
- Always:
  * Commit one file at a time inside the delimiters.
  * Use descriptive Conventional Commit messages with emojis.
  * Avoid staging directories — enumerate files inside them.
  * Maintain the order required by the repository owner, but commit granularity must remain one file per commit.

### How to Commit Changes to `scripts/checkin.sh`
If `scripts/checkin.sh` is listed in `.gitignore` (to avoid accidental commits when editing the CHECKIN block):
- By default, changes to the file will not be staged or committed.
- To commit intentional changes (e.g., bug fixes to helper functions outside the delimiter block):
  1. Explicitly stage the file using:
     ```bash
     git add scripts/checkin.sh
     ```
     or, if it is completely untracked:
     ```bash
     git add -f scripts/checkin.sh
     ```
  2. Commit as normal with a descriptive message:
     ```bash
     git commit -m "fix(checkin): 🐛 correct helper function behavior in commit script"
     ```
- This workflow ensures:
  * Delimiter block changes stay local by default.
  * Core helper logic improvements are versioned deliberately.
- **Note:** Only explicitly add and commit this file when changes are truly meant to be shared with all practitioners.

### Common Pitfalls Observed
During this session, some files under `internal/` were missed in the updated script:
- Newly added directories or template files (`internal/cmd/graph/`, `internal/cmd/pr/create/create_dry_run.tpl`, `internal/cmd/pr/vote/`) were not included.

**Root Cause**:
- The update relied solely on the current visible modified/known files but overlooked untracked directories/templates.
- Possible bias toward previously known command structures — unfamiliar new modules were omitted.

**Preventive Measures**:
- After listing changes with `git status --porcelain internal/`, carefully include both `M` (modified) and `??` (untracked) items.
- Explicitly add any new directories or file types, even if they don't fit existing patterns (e.g., `.tpl`, scaffolding directories).
- Double‑check for untracked folders or non‑Go files — they may need commits and messages.
- Consider grouping new modules appropriately in the commit sequence.
- **Always commit one file at a time**:
  - Do **not** list a directory in `stage_if_exists`; that will stage all contents and commit them with the same message.
  - Instead, enumerate each file inside directories and add separate `stage_if_exists` + `commit_if_staged` calls for each.
  - This ensures granular commit history, aligning with the "one file per commit" requirement.
  - When adding new directories, recursively list files and integrate individually into the script in proper sequence.
