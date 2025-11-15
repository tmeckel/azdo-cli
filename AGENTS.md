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
- **Indentation During Drafts:** Cosmetic indentation mismatches are acceptable while implementing changes. Final formatting is applied with `gofumpt` after coding is complete, so focus on correctness first.

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
    - **CRITICAL: Verify Data Types:** Before using API response data, always inspect the struct definitions in the `vendor/github.com/microsoft/azure-devops-go-api/azuredevops/v7/` directory. Mismatched types (e.g., `int64` vs `uint64`) between the API struct and your command's structs will cause compilation errors.
- **Output:** Use `ctx.Printer(format)` and repository’s standard `printer` helpers to format output in table or JSON, following patterns from existing commands. For commands that make API calls, always wrap the logic with `ios.StartProgressIndicator()` and `defer ios.StopProgressIndicator()` to provide feedback to the user. When not outputting JSON, provide a clean, human-readable table using the `ctx.Printer("list")` helper for list-like output. Choose the most relevant columns to display. Always call `ios.StopProgressIndicator()` **before** creating any output on the command line.
- **Testing:** Create mocks for the relevant client interface methods under `internal/mocks` and write hermetic, table-driven tests alongside the command.

### Handling Missing Azure DevOps SDK Clients

When a required Azure DevOps client is not available in the vendored Go SDK:

1. Confirm the absence by searching the upstream SDK using the GitHub MCP server (`https://github.com/microsoft/azure-devops-go-api/blob/dev/azuredevops/v7`).
2. Extend `type ClientFactory interface` in `internal/azdo/connection.go` with the new client method signature.
3. Ask the user to run `go mod tidy` followed by `go mod vendor` after the interface additions (the sandbox cannot do this automatically).
4. Add a matching mock generation entry to `scripts/generate_mocks.sh`.
5. Let the user run `bash ./scripts/generate_mocks.sh`
6. Implement the factory method in `internal/azdo/factory.go` so the new client can be constructed via existing connection plumbing.

Do not hand-roll HTTP calls if an SDK client can be introduced through this process.

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

To ensure high-quality, production-ready code and prevent common errors, adhere to the following guidelines when generating or modifying Go code:

### Explicit Import Management

- **Mandate:** When generating or modifying Go code, **always explicitly list and verify all required import statements**. Before writing the file, perform a dry run or a mental check to ensure all types, functions, and packages used in the new/modified code are correctly imported.

- **Detail:** Ensure imports for standard library packages (e.g., `fmt`, `strings`, `context`), third-party libraries (e.g., `github.com/spf13/cobra`, `github.com/MakeNowJust/heredoc`), and internal project modules (e.g., `github.com/tmeckel/azdo-cli/internal/cmd/util`, `github.com/tmeckel/azdo-cli/internal/azdo`) are present. If unsure, err on the side of including common imports for the context.

### Idiomatic Go Code & Error Handling

- **Context & IOStreams:** When interacting with `util.CmdContext`, always retrieve `IOStreams` and `Prompter` into local variables to handle potential errors immediately.
    ```go
    // GOOD:
    // iostreams, err := ctx.IOStreams()
    // if err != nil { return err }
    // p, err := ctx.Prompter()
    // if err != nil { return err }

    // BAD:
    // if !ctx.IOStreams().CanPrompt() { ... }
    ```
- **Safely Dereference Pointers:** The Azure DevOps API often returns pointers for fields that can be null. To prevent `nil pointer dereference` panics, use the generic helper `types.GetValue[T](ptr *T, defaultVal T) T`. This is the preferred way to safely access the value of a pointer.

    ```go
    // BAD: This will panic if project.Description is nil
    // description := *project.Description

    // GOOD: This safely returns the description or an empty string
    description := types.GetValue(project.Description, "")
    ```
-   **User Cancellation:** For operations that can be cancelled by the user (e.g., confirmation prompts), prefer returning `util.ErrCancel` over `util.SilentExit` to clearly distinguish user-initiated cancellations from other silent exits.
