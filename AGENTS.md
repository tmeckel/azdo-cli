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

## Commit & Pull Request Guidelines

- **Commits:** Adhere to Conventional Commits specification (e.g., `feat: ...`, `fix: ...`, `chore: ...`, `refactor: ...`).
- **Pull Requests:** Clearly describe changes, rationale, and how to reproduce/verify; reference linked issues.
- **CI Checks:** Run `make lint` and `go test ./...` before submission; update `docs/` if CLI changes.

## Security & Configuration Tips

- **Secrets Management:** Never commit secrets. Use `AZDO_TOKEN` for headless runs; `azdo auth login` uses OS keyring for token storage by default.
- **Default Organization:** Set via `AZDO_ORGANIZATION` or config files under `${AZDO_CONFIG_DIR:-~/.config/azdo}`.

## Agent-Specific Instructions

- **Checklist:** Before making changes, begin with a concise checklist (3-7 conceptual bullets) outlining intended actions. Skip if the change is trivial
- **Change Scope:** Keep changes focused and consistent with existing structure and naming.
- **Patch Size:** Favor small, isolated patches and update/add nearby tests where relevant.
- **Validation:** After code edits, validate results in 1-2 lines and proceed or self-correct if validation fails.
- **Code Edits:** Explicitly state assumptions before edits, create or run minimal tests when possible, and produce ready-to-review diffs following the repository style.
- **Use Gemini:** You act as a coordinator for Gemini MCP server to implement the new command. Gemini acts on your commands. You never inspect the code base on your own. You always call the Gemini MCP server for this if required.
- **Code Scope:** When you create, change or fix tests you only work on tests. You don't change any other code. When required prompt the user to deviate from that instruction.
- **Context7:** Always use context7 when I need code generation, setup or configuration steps, or library/API documentation. This means you should automatically use the Context7 MCP tools to resolve library id and get library docs without me having to explicitly ask.
