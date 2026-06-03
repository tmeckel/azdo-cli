Developer: # Repository Guidelines

## Project Structure & Module Organization

- **Source Code:** Located in `internal/` (core packages: `cmd/*`, `azdo`, `config`, `git`, `iostreams`, `docs`, etc.).
- **CLI Entrypoints:** Main binary at `cmd/azdo/azdo.go`, documentation generator at `cmd/gen-docs/`.
- **Documentation:** Generated Markdown in `docs/` (output from Cobra command hierarchy).
- **Tests:** Placed alongside production code as `*_test.go`.
- **Vendoring:** Dependencies locked in `vendor/`.

## Build, Test, and Development Commands

- **Build CLI:** `make build` (produces `azdo` binary).
- **Lint:** `make lint` (runs golangci-lint based on `.golangci.yml`).
- **Testing:** `go test ./...` (use `TIMEOUT=...` for custom timeout).
- **Documentation:** `make docs` (rebuilds `docs/` via `cmd/gen-docs`).
- **Housekeeping:** `make tidy` (`go mod tidy`), `make clean` (removes binaries/distribution files).
- **Run Locally:** `go run cmd/azdo/azdo.go --version` (no installation required).

## Go Coding Skills

This project relies on the [samber/cc-skills-golang](https://github.com/samber/cc-skills-golang) skill packs for general Go conventions. The full skill catalog, error-rate impact per skill, and the recommended ⭐️ list are documented in the [samber README](https://github.com/samber/cc-skills-golang#-skills). Install with the [vercel-labs/skills](https://github.com/vercel-labs/skills) CLI:

```bash
# Inspect which Go skills are currently installed
npx skills list | grep golang-

# Install a single skill
npx skills add samber/cc-skills-golang --skill golang-code-style -y
```

**Workflow:**

1. Before writing or modifying Go code, run `npx skills list | grep golang-` to see which skills are installed in this project or globally. The agent loads them automatically based on description matching; if `golang-how-to` is installed it also force-loads relevant secondary skills (e.g. Cobra review → `golang-spf13-cobra` + `golang-cli` + `golang-error-handling`).
2. Treat the installed samber skills as the source of truth for general Go rules (style, naming, error wrapping, nil safety, testing patterns, concurrency, context propagation, etc.).
3. Apply the project-specific rules in the sections below only where they **deviate** from samber. Every such section declares `> Supersedes samber/cc-skills-golang@<skill> for this project.` at the top — samber's ⚙️ override mechanism is honored automatically.
4. If a section below does not declare a supersession, the samber skills win on that topic.

> **When in doubt:** trust the samber skill that triggers. Project-specific sections exist to add or restrict, not to re-teach.

## Coding Style & Naming Conventions

> Supersedes `samber/cc-skills-golang@golang-code-style`, `@golang-naming`, and `@golang-error-handling` for this project.

For the general rules (gofmt, goimports, MixedCaps, `Get`-prefix, `errors.Is/As`, `%w` wrapping), defer to the installed samber skills. Project-specific deviations:

- **Linting:** `make lint` runs `golangci-lint` against `.golangci.yml`. Always run before submission.
- **Logging:** Use `zap.L()` with structured messages. Do not introduce `log` or `slog` without approval.
- **Error wrapping:** Always propagate with `fmt.Errorf("...: %w", err)`.
- **Variables:** Variable names must never collide with any import or the name of a Go package.
- **CLI Flags:** Use kebab-case (e.g., `--organization-url`).
- **Multi-value flag sentinel:** When supporting "remove all" semantics on list flags, reserve `*` as the exclusive sentinel. Commands must reject combinations like `--remove-label foo,*` and treat a lone `*` as "remove every existing entry."
- **Editing tools:** Modify files using git-aware patches (e.g., `apply_patch`). Do not use ad-hoc scripts (Python, sed) to edit tracked files — diffs must stay reviewable.
- **Indentation during drafts:** Cosmetic indentation mismatches are acceptable while implementing changes. Final formatting is applied with `gofumpt` after coding is complete, so focus on correctness first.

## Testing Guidelines

> Supersedes `samber/cc-skills-golang@golang-testing` and `@golang-stretchr-testify` for this project.

For the general patterns (table-driven, parallel, fuzzing, coverage, `assert` vs `require`), defer to the installed samber skills. Project-specific deviations:

- **Frameworks:** `testing` + `testify` (`require` for fatal preconditions, `assert` for non-fatal).
- **Hermetic:** All tests **must** be hermetic and use mocks under `internal/mocks`. **Never** call the Azure DevOps REST API directly in tests — generate or extend a mock instead.
- **REST API commands:** For commands that touch Azure DevOps REST API endpoints (`internal/cmd`), always add black-box tests informed by Azure DevOps REST API 7.1 documentation.
- **Imports:** Review and understand structs and functions in `vendor/` to maintain correct imports.
- **Coverage:** Add tests for new features and edge cases (e.g., authentication, URL parsing, remote operations).

For a complete guide, refer to [TESTING.md](./TESTING.md).

## Commit & Pull Request Guidelines

- **Commits:** Adhere to Conventional Commits specification (e.g., `feat: ...`, `fix: ...`, `chore: ...`, `refactor: ...`).
- **Pull Requests:** Clearly describe changes, rationale, and how to reproduce/verify; reference linked issues.
- **CI Checks:** Run `make lint` and `go test ./...` before submission; update `docs/` if CLI changes.

## Security & Configuration Tips

- **Secrets Management:** Never commit secrets. Use `AZDO_TOKEN` for headless runs; `azdo auth login` uses OS keyring for token storage by default.
- **Default Organization:** Set via `AZDO_ORGANIZATION` or config files under `${AZDO_CONFIG_DIR:-~/.config/azdo}`.

## Agent-Specific Instructions

### Plan Adherence & Deviation Approval

- If you propose a plan and the user approves it, you must follow it; any material deviation (approach, dependencies, SDK-vs-REST, output shape) requires you to stop and ask for approval before implementing.
- If a planned step is blocked by sandbox/network/tooling constraints, do not “work around” by changing the approach; pause and ask for approval (or for the user to perform required external steps) instead.
- If you need to create directories or delete/move files, do not do it yourself; stop and ask the user to perform the action, then continue once confirmed.
- Do not introduce placeholder/stub implementations (e.g., TODOs, empty helpers, or temporary `return nil`) to “get unstuck”; if information is missing, stop and ask the user for guidance/approval.
- Prefer MCP tools (GitHub MCP / msdocs / context7) over direct network fetches (`curl`, etc.); if MCP cannot provide what’s needed, try network fetches or internet search. If this does not work either, ask the user to fetch/provide the information instead.

### Implementing New CLI Commands

- **Location & Structure:** Place new command code in an appropriate subdirectory under `internal/cmd/<category>/<command>/`, matching the CLI hierarchy. For example, `azdo repo create` → `internal/cmd/repo/create/create.go`.
- **Factory Function:** Define a single factory function named `NewCmd(ctx util.CmdContext) *cobra.Command` — do not prefix with category names (old `NewCmdRepoX` style is deprecated).
- **Options Handling:** Use an unexported `opts` struct to bind CLI flags. Keep parsing and validation in `RunE` minimal, delegating logic to a separate `runX` function.
- **CmdContext Usage:** Always use the injected `util.CmdContext` to retrieve `IOStreams`, configuration (`ctx.Config()`), connection (`ctx.ConnectionFactory()`), and typed API clients via `ctx.ClientFactory()`.
- **Vendored API:** Access Azure DevOps endpoints via the vendored `azuredevops/v7` client packages instead of raw HTTP calls. Build the appropriate `Args` structs and call the client method (e.g., `git.Client.CreateRepository`).
    - **CRITICAL: Verify Data Types:** Before using API response data, always inspect the struct definitions in the `vendor/github.com/microsoft/azure-devops-go-api/azuredevops/v7/` directory. Mismatched types (e.g., `int64` vs `uint64`) between the API struct and your command's structs will cause compilation errors.
- **Output:** Use `ctx.Printer(format)` and the standard printer helpers to format table or JSON output. When not emitting JSON, prefer `ctx.Printer("list")` for clear tables and pick the most relevant columns for the scenario.

#### Progress Indicator Usage
- Start the progress indicator immediately after successfully obtaining `IOStreams` so users see activity while you parse inputs or build clients.
- Immediately `defer ios.StopProgressIndicator()` to guarantee cleanup on every return path.
- If you need to print to stdout/stderr before the deferred stop executes, call `ios.StopProgressIndicator()` first to keep progress text from interleaving with user-visible output.
- **Testing:** Create mocks for the relevant client interface methods under `internal/mocks` and write hermetic, table-driven tests alongside the command.

### Handling Missing Azure DevOps SDK Clients

When a required Azure DevOps client is not available in the vendored Go SDK:

1. Confirm the absence by searching the upstream SDK using the GitHub MCP server (`https://github.com/microsoft/azure-devops-go-api/blob/dev/azuredevops/v7`).
2. Extend `type ClientFactory interface` in `internal/azdo/connection.go` with the new client method signature.
3. Ask the user to run `go mod tidy` followed by `go mod vendor` after the interface additions (the sandbox cannot do this automatically).
4. Add a matching mock generation entry to `scripts/generate_mocks.sh`.
5. Let the user run `bash ./scripts/generate_mocks.sh`
6. Implement the factory method in `internal/azdo/factory.go` so the new client can be constructed via existing connection plumbing.

Do not hand-roll HTTP calls or add new `internal/azdo/extensions` methods as a shortcut when an SDK client can be introduced through this process; if the user-approved plan is to add an SDK client, follow the steps above or stop and ask for approval to change approach.

### Implementing Commands with JSON and Table/Plain Output

- **JSON and Table/Plain output are handled via separate code paths.**
- Use `util.AddJSONFlags(cmd, &opts.exporter, ...)` in `NewCmd` to register JSON-related flags (`--json`, `--jq`, `--template`). This populates the `opts.exporter` field when a user specifies one of those flags. The string slice you pass **must** list every JSON field you expose (matching the struct tag names) so users can filter output predictably.

- **JSON Output Logic:**
  - In the command's `run...` function, check if `opts.exporter != nil`.
  - If true, this indicates the user wants JSON output.
  - Define a dedicated view struct (or slice of structs) that represents the JSON surface you intend to support. Field names must match the strings you register with `util.AddJSONFlags` and every optional field should use a pointer type with `json:"...,omitempty"` so unset values disappear from the payload.
  - Populate that view struct from the SDK model (write small helper functions when mapping requires normalization—e.g., formatting `azuredevops.Time`, collapsing identities, adding derived counts). Avoid returning the raw SDK types directly; surface only the columns you are committed to supporting.
  - Call `opts.exporter.Write(ios, result)` to serialize the struct and print it.

- **Table/Plain Output Logic:**
  - This is the default path, executed when `opts.exporter == nil`.
  - **For creating horizontal tables:**
    1. Get a printer: `tp, err := ctx.Printer("list")`.
    2. Define all column headers: `tp.AddColumns("Header1", "Header2", ...)`.
    3. Finalize the header row: `tp.EndRow()`.
    4. For each data row, add cell values in order: `tp.AddField(value1)`, `tp.AddField(value2)`, etc.
    5. Finalize the data row: `tp.EndRow()`.
    6. Render the table: `tp.Render()`.
    - **CRITICAL:** `AddField` populates a cell in a horizontal row corresponding to a column defined by `AddColumns`. It is **not** for creating vertical key-value lists (e.g., `Label: Value`). Do not pass a label to `AddField`.
  - **For simple text output** (e.g., a success message), `fmt.Fprintf(ios.Out, "...")` is acceptable.
- **Documentation:**
    - Add CLI help examples for using `--json` and `--format table`.
    - Run `make docs` to regenerate markdown so output options appear in generated documentation.

- **Parameter Derivation:** Before defining CLI flags/args for a new command, analyze the corresponding method in the vendored Azure DevOps Go API under `vendor/github.com/microsoft/azure-devops-go-api/azuredevops/v7/<service>/client.go` (or related file). Review its function signature and the associated `Args` struct:
  - Map required struct fields or parameters to required CLI flags or positional arguments.
  - Map optional fields to optional CLI flags, providing sensible defaults from environment variables (`AZDO_*`) or configuration where applicable.
  - Use repository conventions for flag naming (kebab-case) and argument positioning.
  - Ensure every CLI parameter has clear help text and appears in regenerated documentation (`make docs`).

- **Organization/Project Positional Argument Parsing:**
  - For commands that operate within a project, accept the target project and optional organization as the first positional argument in the form `[ORGANIZATION/]PROJECT`.
  - Parse by splitting on `/`:
    - **One segment** → project only; retrieve organization from default config.
    - **Two segments** → first is organization, second is project.
    - **Any other segment count** → return a flag/argument error.
  - If organization is omitted and no default is configured, return an error stating that no organization was specified or configured.
  - Follow patterns established in existing commands (e.g., `internal/cmd/repo/list/list.go`) for parsing and validation.

- **Pagination for List Operations:**
  - When using any Azure DevOps SDK `List*` method that supports continuation tokens, loop until the `ContinuationToken` in the response is nil or empty.
  - Append results across all pages before applying filters or selections.
  - Break the loop only after exhausting all pages, or if the command supports `--max-items [int>0]` and the user specified a value stop after the max value

- **Confirmation for Destructive Operations:**
  - Destructive commands must prompt the user for confirmation unless a `--yes` flag is provided.
  - On cancellation, return `util.ErrCancel`.

- **Nil Handling for API Fields:**
  - Explicitly check for nil pointer fields returned from API calls before use.
  - Use `types.GetValue` for safe dereferencing or return an error if the value is required and missing.

- **Debug Logging:**
  - Use `zap.L().Debug` to log critical decision points (e.g., parsing format detected, scope descriptor resolutions, API call stages).

- **Aliases:**
  - For common destructive commands, provide short command aliases such as `d`, `del`, `rm` to improve ergonomics.
  - For list commands, provide aliases such as `l`, `ls`
  - For commands that create objects provide aliases like `c`, `cr`

- **Working with Scope Descriptors:**
  - For project-scoped operations, you must first fetch the project object using the `core.Client`.
  - Then, you must use the `graph.Client`'s `GetDescriptor` method with the project's `StorageKey` (ID) to get the correct scope descriptor.
  - The `graph.Client`'s `GetDescriptor` can be used to get the storage descriptor for various objects like projects, users, groups etc.

### Wiring Commands into the CLI Hierarchy

- **Single-Level Addition (Leaf to Existing Group):**
  - Implement the new leaf command in the correct parent group’s subdirectory, using `NewCmd(ctx)`.
  - Import and register it in the parent’s `NewCmd` via `parentCmd.AddCommand(child.NewCmd(ctx))`.

- **New Subgroup Under Existing Group:**
  - Create a new directory for the subgroup under the group’s folder.
  - Implement `NewCmd(ctx)` for the subgroup (acts as a parent for its own children).
  - Register the subgroup in the immediate parent’s `NewCmd` with `AddCommand`.
  - Add leaf commands under the subgroup and register them similarly.

- **New Top-Level Group:**
  - Implement the group’s `NewCmd(ctx)` in `internal/cmd/<group>/<group>.go`.
  - Add it to the root command in `internal/cmd/root/root.go` via `rootCmd.AddCommand(group.NewCmd(ctx))`.

- **Multi-Level Nesting:**
  - For deeper hierarchies (e.g., `root → graph → viz → generate`), ensure **each parent in the chain** calls `AddCommand()` for its direct children.
  - Create folders reflecting the hierarchy (`internal/cmd/graph/viz/generate/`).
  - Provide a `NewCmd` function for every level that initializes its Cobra command and wires its children.

- **Documentation Update:** After adding any new command at any level, run `make docs` to regenerate CLI documentation so all new commands appear in `docs/`.

- **Checklist:** Before making changes, begin with a concise checklist (3-7 conceptual bullets) outlining intended actions. Skip if the change is trivial
- **Change Scope:** Keep changes focused and consistent with existing structure and naming.
- **Patch Size:** Favor small, isolated patches and update/add nearby tests where relevant.
- **Patch Size:** Favor small, isolated patches and update/add nearby tests where relevant. If a planned step becomes blocked, stop and ask instead of substituting a different implementation strategy.
- **Validation:** After code edits, validate results in 1-2 lines and proceed or self-correct if validation fails.
- **Code Edits:** Explicitly state assumptions before edits, create or run minimal tests when possible, and produce ready-to-review diffs following the repository style.
- **Code Scope:** When you create, change or fix tests you only work on tests. You don't change any other code. When required prompt the user to deviate from that instruction.
- **Context7:** Always use context7 when I need code generation, setup or configuration steps, or library/API documentation. This means you should automatically use the Context7 MCP tools to resolve library id and get library docs without me having to explicitly ask.

### Implementing Commands with JSON and Table/Plain Output

- **JSON and Table/Plain output are handled via separate code paths.**
- Use `util.AddJSONFlags(cmd, &opts.exporter, ...)` in `NewCmd` to register JSON-related flags (`--json`, `--jq`, `--template`). This populates the `opts.exporter` field when a user specifies one of those flags, and the provided slice must enumerate every JSON field (matching the struct tags) that the command supports so consumers can filter reliably.

- **JSON Output Logic:**
  - In the command's `run...` function, check if `opts.exporter != nil`.
  - If true, this indicates the user wants JSON output.
  - Define a dedicated view struct (or slice of structs) with explicit `json:"..."` tags and register the matching field names with `util.AddJSONFlags`. Use pointer types plus `omitempty` for optional fields so unset data is omitted, and map values from the SDK into this view (formatting times, flattening identities, computing derived helpers). Do not expose raw SDK structs in the JSON response.
  - Call `opts.exporter.Write(ios, result)` to serialize the struct and print it.

- **Table/Plain Output Logic:**
  - This is the default path, executed when `opts.exporter == nil`.
  - For tabular data, get a printer with `tp, err := ctx.Printer("list")`. Use `tp.AddColumns()`, `tp.AddField()`, and `tp.EndRow()` to build the table, then call `tp.Render()`.

## Code Generation Best Practices

> Supersedes `samber/cc-skills-golang@golang-code-style`, `@golang-error-handling`, `@golang-safety`, and `@golang-structs-interfaces` for this project.

For the general Go rules (gofmt, goimports, error wrapping, nil safety, struct/interface design, embedding, pointer-vs-value receivers), defer to the installed samber skills. The following project-specific helpers and imports must be used in addition:

### Helpers (use these — do not reinvent)

- **`types.GetValue[T]`** (`internal/types`): Safe pointer dereference. Required for Azure DevOps API fields that may be nil — do not use `*ptr` directly.
- **`types.MapSlice` / `types.MapSlicePtr`** (`internal/types`): Generic slice transforms that handle nil pointers. Use instead of manual mapping loops.
- **`util.AddJSONFlags(cmd, &opts.exporter, ...)`**: Register `--json` / `--jq` / `--template` on every command that emits structured output. The string slice you pass **must** list every JSON field you expose.
- **`util.ErrCancel`**: Return this — not `util.SilentExit` — when the user cancels a confirmation prompt.
- **`util.CmdContext`**: Always retrieve `IOStreams`, `Prompter`, `Config`, `ConnectionFactory`, and `ClientFactory` via this. Never reach for globals.

### Imported packages (use only the canonical set)

- Standard library: `fmt`, `strings`, `context`
- Third-party: `github.com/spf13/cobra`, `github.com/MakeNowJust/heredoc`, `go.uber.org/zap`
- Internal: `github.com/tmeckel/azdo-cli/internal/cmd/util`, `github.com/tmeckel/azdo-cli/internal/azdo`, `github.com/tmeckel/azdo-cli/internal/types`
