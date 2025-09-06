# Repository Guidelines

## Project Structure & Module Organization
- Source: `internal/` (core packages: `cmd/*`, `azdo`, `config`, `git`, `iostreams`, `docs`, etc.).
- CLI entrypoints: `cmd/azdo/azdo.go` (binary), `cmd/gen-docs/` (docs generator).
- Docs: `docs/` (generated Markdown from the Cobra tree).
- Tests: co-located as `*_test.go` within packages.
- Vendoring: `vendor/` (locked deps).

## Build, Test, and Development Commands
- Build CLI: `make build` → produces `azdo`.
- Lint: `make lint` → runs golangci-lint using `.golangci.yml`.
- Tests: `go test ./...` (or `TIMEOUT=... go test ./...`).
- Docs: `make docs` → regenerates `docs/` via `cmd/gen-docs`.
- Housekeeping: `make tidy` (module tidy), `make clean` (remove binaries/dist).
- Run locally (no install): `go run cmd/azdo/azdo.go --version`.

## Coding Style & Naming Conventions
- Language: Go 1.22; format with `gofmt`/`goimports`; keep diffs minimal.
- Linting: adhere to `golangci-lint` rules; wrap errors with `%w`.
- Packages lower-case; exported identifiers `CamelCase`; files lower_snake.
- CLI flags use kebab-case (e.g., `--organization-url`).
- Logging: use `zap.L()`; prefer structured messages and `%w` for error chains.

## Testing Guidelines
- Frameworks: standard `testing` + `testify` utilities.
- Conventions: `*_test.go`, functions `TestXxx`; prefer table-driven tests.
- Run: `go test ./...`; keep tests hermetic (avoid network; use fixtures under `internal/*/testdata`).
- Add tests for new behavior and edge cases (auth, URL parsing, remotes).

## Commit & Pull Request Guidelines
- Commits: follow Conventional Commits (e.g., `feat: ...`, `fix: ...`, `chore: ...`, `refactor: ...`).
- PRs: include clear description, rationale, and reproduction/verification steps; link issues.
- CI hygiene: run `make lint` and `go test ./...` before submitting; update `docs/` if CLI surface changes.

## Security & Configuration Tips
- Never commit secrets. Use `AZDO_TOKEN` for headless runs; interactive `azdo auth login` stores tokens in OS keyring by default.
- Default org can come from `AZDO_ORGANIZATION` or config files under `${AZDO_CONFIG_DIR:-~/.config/azdo}`.

## Agent-Specific Instructions
- Keep changes focused; match existing structure and naming.
- Prefer small, surgical patches; update or add nearby tests.
- If you modify flags/commands, regenerate docs with `make docs`.
