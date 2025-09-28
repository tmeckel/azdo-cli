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

## Coding Style & Naming Conventions

- **Language:** Go 1.22; use `gofmt` and `goimports` for code formatting; keep diffs minimal.
- **Linting:** Follow `golangci-lint` guidance; wrap errors using `%w` for error chains.
- **Naming:** Packages use lowercase; exported identifiers in CamelCase; files use lower_snake_case.
- **CLI Flags:** Use kebab-case (e.g., `--organization-url`).
- **Logging:** Use `zap.L()` with structured messages; prefer `%w` for wrapping errors.
- **Variables:** Variable names must never collide with any imports or name of GO packages

## Testing Guidelines

- **Frameworks:** Use standard `testing` package and `testify` tools.
- **Conventions:** Place tests in `*_test.go`; follow `TestXxx` function format; prefer table-driven tests.
- **Execution:** tests **must** be hermetic and use mocks (`internal/mocks`). Create new mocks as needed. All tests must use a simulated API via mocks, no calling the Azure DevOps REST API directly.
- **Coverage:** Add tests for new features and edge cases (e.g., authentication, URL parsing, remote operations).
- **Imports:** Review and understand structs and functions from the `vendor` directory to maintain correct imports.
- **REST API Commands:** For commands that interact with Azure DevOps REST API endpoints (`internal/cmd`), always add black box tests informed by Azure DevOps REST API 7.1 documentation.

For a complete guidance on how to implement tests refer to [TESTING.md](./TESTING.mds)

## Commit & Pull Request Guidelines

- **Commits:** Adhere to Conventional Commits specification (e.g., `feat: ...`, `fix: ...`, `chore: ...`, `refactor: ...`).
- **Pull Requests:** Clearly describe changes, rationale, and how to reproduce/verify; reference linked issues.
- **CI Checks:** Run `make lint` and `go test ./...` before submission; update `docs/` if CLI changes.

## Security & Configuration Tips

- **Secrets Management:** Never commit secrets. Use `AZDO_TOKEN` for headless runs; `azdo auth login` uses OS keyring for token storage by default.
- **Default Organization:** Set via `AZDO_ORGANIZATION` or config files under `${AZDO_CONFIG_DIR:-~/.config/azdo}`.

## Agent-Specific Instructions

### Implementing New CLI Commands

- **Location & Structure:** Place new command code in an appropriate subdirectory under `internal/cmd/<category>/<command>/`, matching the CLI hierarchy. For example, `azdo repo create` → `internal/cmd/repo/create/create.go`.
- **Factory Function:** Define a single factory function named `NewCmd(ctx util.CmdContext) *cobra.Command` — do not prefix with category names (old `NewCmdRepoX` style is deprecated).
- **Options Handling:** Use an unexported `opts` struct to bind CLI flags. Keep parsing and validation in `RunE` minimal, delegating logic to a separate `runX` function.
- **CmdContext Usage:** Always use the injected `util.CmdContext` to retrieve `IOStreams`, configuration (`ctx.Config()`), connection (`ctx.ConnectionFactory()`), and typed API clients via `ctx.ClientFactory()`.
- **Vendored API:** Access Azure DevOps endpoints via the vendored `azuredevops/v7` client packages instead of raw HTTP calls. Build the appropriate `Args` structs and call the client method (e.g., `git.Client.CreateRepository`).
- **Output:** Use `ctx.Printer(format)` and repository’s standard `printer` helpers to format output in table or JSON, following patterns from existing commands.
- **Testing:** Create mocks for the relevant client interface methods under `internal/mocks` and write hermetic, table-driven tests alongside the command.
- **Output Formats (Table & JSON):**
  - **JSON Support:**
    - Use `util.AddJSONFlags(cmd, &exporter, []string{<fields>})` in `NewCmd` to register `--json`, `--jq`, and `--template` options.
    - Maintain an `util.Exporter` field in the command’s `opts` struct to capture JSON export configuration.
    - Pass the relevant response object to `exporter.Export()` when non-nil.
    - Declare valid JSON fields based on the struct’s exported properties. Help annotations are added automatically by `AddJSONFlags`.
  - **Table/Plain Output:**
    - Default to human-friendly plain console output for single-object results.
    - Use `ctx.Printer(format)` with `tp.AddColumns`/`tp.AddField` for tabular format when `--format table` is set or for multiple rows.
    - Keep column names consistent (e.g., "ID", "Name", "WebUrl").
  - **Format Flag:**
    - Always include a `--format` flag via `util.StringEnumFlag` with supported formats (at least `json` and `table`).
    - Output decision order:
      1. If `exporter != nil` → JSON output.
      2. Else if table explicitly requested or multiple rows → table output.
      3. Else → plain text summary.
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
- **Validation:** After code edits, validate results in 1-2 lines and proceed or self-correct if validation fails.
- **Code Edits:** Explicitly state assumptions before edits, create or run minimal tests when possible, and produce ready-to-review diffs following the repository style.
- **Use Gemini:** You act as a coordinator for Gemini MCP server to implement the new command. Gemini acts on your commands. You never inspect the code base on your own. You always call the Gemini MCP server for this if required.
- **Code Scope:** When you create, change or fix tests you only work on tests. You don't change any other code. When required prompt the user to deviate from that instruction.
- **Context7:** Always use context7 when I need code generation, setup or configuration steps, or library/API documentation. This means you should automatically use the Context7 MCP tools to resolve library id and get library docs without me having to explicitly ask.

### Implementing Commands with JSON and Table Output

- Do **NOT** call a nonexistent `Exporter.Export` method; in this codebase `util.Exporter` does not implement such a function. JSON and table output are unified through the `ctx.Printer` abstraction.
- For consistent output handling, define a `--format` flag using `util.StringEnumFlag` with `table` default and `json` as an option.
- Initialize a printer in `RunE` via:
  ```go
  tp, err := ctx.Printer(opts.format)
  if err != nil {
      return err
  }
  ```
- Populate table output with `tp.AddColumns(...)`, `tp.AddField(...)`, and `tp.EndRow()`; json formatting will use the same `tp.Render()` call.
- Treat `ios.Out` from `IOStreams` as an `io.Writer`, not a callable function: write directly with `fmt.Fprintln(ios.Out, ...)` or delegate to the printer’s `Render` method.
- Always call `tp.Render()` at the end of output preparation to emit results, whether in JSON or table format.
