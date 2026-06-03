# Contributing

❣ We love pull requests from everyone !

## All code changes happen through Pull Requests

Pull requests are the best way to propose changes to the codebase. We actively
welcome your pull requests:

1. Fork the repo and create your branch from `master`.
2. If you've added code that should be tested, add tests.
3. If you've added code that need documentation, update the documentation.
4. Write a [good commit message](http://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html).
5. Issue that pull request!

If you've never written Go in your life, then join the club! Go is widely
considered an easy-to-learn language, so if you're looking for an open source
project to gain dev experience, you've come to the right place.

## Running in a Github Codespace

If you want to start contributing to `azdo` with the click of a button, you can
open the `azdo` codebase in a Codespace. First fork the repo, then click to
create a codespace:

This allows you to contribute to `azdo` without needing to install anything on
your local machine. The Codespace has all the necessary tools and extensions
pre-installed.

## Code of conduct

Please note by participating in this project, you agree to abide by the [code of conduct](./CODE-OF-CONDUCT.md).

## Any contributions you make will be under the MIT Software License

In short, when you submit code changes, your submissions are understood to be
under the same [MIT License](./LICENSE) that
covers the project.

## Report bugs using Github's [issues](https://github.com/tmeckel/azdo-cli/issues)

We use GitHub issues to track public bugs. Report a bug by [opening a new
issue](https://github.com/tmeckel/azdo-cli/issues/new); it's that easy!

## Go

This project is written in Go. Go is an opinionated language with strict idioms, but some of those idioms are a little extreme. Some things we do differently:

1. There is no shame in using `self` as a receiver name in a struct method. In fact we encourage it
2. There is no shame in prefixing an interface with 'I' instead of suffixing with 'er' when there are several methods on the interface.
3. If a struct implements an interface, we make it explicit with something like:

```go
var _ MyInterface = &MyStruct{}
```

This makes the intent clearer and means that if we fail to satisfy the interface we'll get an error in the file that needs fixing.

### Code Formatting

To check code formatting [gofumpt](https://pkg.go.dev/mvdan.cc/gofumpt#section-readme) (which is a bit stricter than [gofmt](https://pkg.go.dev/cmd/gofmt)) is used.
VSCode will format the code correctly if you tell the Go extension to use `gofumpt` via your [`settings.json`](https://code.visualstudio.com/docs/getstarted/settings#_settingsjson)
by setting [`formatting.gofumpt`](https://github.com/golang/tools/blob/master/gopls/doc/settings.md#gofumpt-bool) to `true`:

```jsonc
// .vscode/settings.json
{
  "gopls": {
    "formatting.gofumpt": true
  }
}
```

To run gofumpt from your terminal go:

```bash
go install mvdan.cc/gofumpt@latest && gofumpt -l -w .
```

## Maintainer onboarding

If you are new to the codebase, start here before making structural changes.

### Mental model

`azdo` is organized as a layered CLI:

1. `cmd/azdo/azdo.go` boots the binary.
2. `internal/cmd/root/root.go` builds the root Cobra command and wires the top-level command tree.
3. `internal/cmd/...` contains command groups and leaf commands.
4. `internal/cmd/util/cmd_context.go` provides the shared runtime context used by commands.
5. `internal/azdo/...` creates Azure DevOps connections and typed SDK clients.
6. `internal/git/...`, `internal/iostreams/...`, and `internal/printer/...` handle repository discovery, terminal UX, and output formatting.

When in doubt, trace a command from the root command down into its `run...` helper and then into the client factory.

### Where to start reading

- `internal/cmd/root/root.go` - best entry point for understanding the CLI surface area.
- `internal/cmd/util/cmd_context.go` - central dependency-injection seam for config, I/O, prompting, printers, repo helpers, and API clients.
- `internal/azdo/connection.go` - interfaces for Azure DevOps connections and typed clients.
- `internal/azdo/factory.go` - concrete implementation for creating org-scoped SDK clients.
- `internal/iostreams/iostreams.go` - pager, TTY, spinner, and stream lifecycle behavior.
- `TESTING.md` - required testing conventions, mock usage, and acceptance-test boundaries.

### Command structure conventions

Most commands follow the same pattern:

- each package exports `NewCmd(ctx util.CmdContext) *cobra.Command`
- a private options struct stores flags and args
- `RunE` does only minimal parsing and validation
- actual behavior lives in a dedicated `run...` helper

Representative examples:

- `internal/cmd/repo/create/create.go`
- `internal/cmd/project/show/show.go`
- `internal/cmd/pr/list/list.go`

Prefer this structure when adding or refactoring commands. It keeps CLI plumbing small and makes testing easier.

### Working with `CmdContext`

`util.CmdContext` is the main abstraction shared across commands. Use it to retrieve:

- `IOStreams()` for stdout, stderr, TTY, and progress behavior
- `Config()` for organization and auth-related settings
- `Prompter()` for interactive flows
- `Printer(...)` for table, list, or JSON output
- `ConnectionFactory()` and `ClientFactory()` for Azure DevOps access
- `RepoContext()` for local-repository-aware operations

Avoid constructing these dependencies ad hoc inside commands unless there is a strong reason. The codebase is designed around the context abstraction.

### Azure DevOps client flow

The usual service path is:

1. Parse organization/project/repo scope in a command.
2. Call `ctx.ClientFactory().<Service>(ctx.Context(), organization)`.
3. Build the vendored Azure DevOps SDK `Args` struct.
4. Call the SDK client method.
5. Map the result into printer or JSON output.

The vendored SDK under `vendor/github.com/microsoft/azure-devops-go-api/azuredevops/v7/` is the source of truth for API types. Check the vendored struct definitions before adding new fields or assumptions to command code or tests.

### Output and JSON conventions

- Prefer `util.AddJSONFlags(...)` for commands that expose structured JSON output.
- Keep JSON and table/plain output as separate code paths.
- When returning JSON, define a stable view struct instead of dumping raw SDK types unless the existing command already intentionally does that.
- For non-JSON output, prefer `ctx.Printer("list")` for tables.

See `internal/cmd/util/json_flags.go` for the common implementation and `internal/cmd/repo/create/create.go` for a representative command-level use of exporter vs table output.

### Progress indicator rules

Progress handling is centralized in `internal/iostreams/iostreams.go`, but each command is responsible for sequencing it correctly.

Use this pattern:

1. Get `IOStreams()`.
2. Start the progress indicator immediately.
3. `defer StopProgressIndicator()`.
4. Stop it explicitly before writing user-visible output.

If you print while the spinner is still active, CLI output can become messy or hard to read.

### Testing workflow

Tests are expected to be hermetic.

- unit tests live next to the code as `*_test.go`
- mocks are generated under `internal/mocks/`
- command tests typically use `gomock`, `testify`, and `iostreams.Test()`
- acceptance tests are opt-in and documented in `TESTING.md`

Before adding a new mock, update `scripts/generate_mocks.sh` and regenerate mocks as described in `TESTING.md`.

### Build, docs, and day-to-day commands

- `make build` - build the CLI
- `make lint` - run `golangci-lint`
- `go test ./...` - run tests
- `make docs` - regenerate CLI docs from the Cobra tree
- `make generate-mocks` - regenerate mocks after interface changes

If you change command flags, examples, or hierarchy, regenerate `docs/` before opening a pull request.

### Hotspots maintainers should watch

- `internal/cmd/root/root.go` - top-level wiring is easy to break when adding commands or aliases.
- `internal/cmd/util/cmd_context.go` - changes here ripple widely through the command tree.
- `internal/azdo/connection.go` and `internal/azdo/factory.go` - client interface changes require matching updates in implementations and mocks.
- `internal/iostreams/iostreams.go` - output and spinner lifecycle bugs can surface as UX regressions.
- pagination helpers such as `internal/azdo/loader.go` - small mistakes here can affect many list commands.

Treat these files as high-leverage areas: small changes can have wide effects.

### Before you open a pull request

- keep the patch focused
- add or update tests for behavior changes
- regenerate docs when CLI output or flags change
- regenerate mocks when interfaces change
- run `make lint` and `go test ./...`
- write a Conventional Commit style message if you are creating commits in the repo workflow

## Improvements

If you can think of any way to improve these docs let us know.
