# Testing Guide

This repo uses standard Go testing with testify and gomock to ensure commands behave correctly against the Azure DevOps REST API v7.1. Tests are hermetic: they do not reach the network and they do not require a live git or Azure DevOps environment.

## Philosophy

- Black‑box first: Author tests from the perspective of the CLI surface and the Azure DevOps REST API documentation, not the current implementation details.
- Spec‑driven: Validate request shapes and behaviors against Microsoft Learn docs for Azure DevOps REST API v7.1 (e.g., fully qualified refs, search criteria, field names, status and pagination semantics).
- Hermetic and deterministic: No network calls; use mocks for all external dependencies (REST clients, identity, git). Avoid global state.
- Small, surgical patches: Each test focuses on one behavior/error path and asserts only what it must.

## Tooling

- Frameworks/libraries:
  - Standard `testing` package
  - `testify` (`assert`, `require`) for clear, expressive assertions
  - `gomock` for mocks and expectations
- Mocks live under `internal/mocks/` and are generated via `mockgen`.
- The Mock library is `go.uber.org/mock/gomock` and must be imported in all test files which uses mocks

## Where tests live

- Tests are colocated with code (`*_test.go`).
- Command tests: `internal/cmd/<group>/<cmd>/*_test.go`.
- Keep test fixtures (if any) under the corresponding package’s `testdata/` subdir.

## Working with mocks

- All external boundaries are mocked:
  - Azure DevOps REST clients (e.g., Git client): `internal/mocks/azdogit_client_mock.go`
  - Identity client: `internal/mocks/identity_client_mock.go`
  - Connection factory/repo context: `internal/mocks/connection_factory_mock.go`, `internal/mocks/repository_mock.go`, `internal/mocks/cmd_context_mock.go`
  - Local git interactions: `internal/mocks/git_command_mock.go`
- Typical pattern:
  - Create a gomock controller in each test and `t.Cleanup(ctrl.Finish)`.
  - Build mocks for command context, repo context, git command, REST clients, identity, and connection factory as needed.
  - Set expectations with `EXPECT()`; use `DoAndReturn` for argument validation.
  - Keep expectations minimal and focused on the behavior under test.

### Mocking Quickstart Checklist

Follow this order every time you add a unit test that touches Azure DevOps clients or command context plumbing:

1. **Create the controller**
   ```go
   ctrl := gomock.NewController(t)
   t.Cleanup(ctrl.Finish)
   ```
2. **Instantiate required mocks** from `internal/mocks` (`CmdContext`, `Config`, `AuthConfig`, `ClientFactory`, specific service clients, printers, prompters, etc.).
3. **Set baseline expectations** with `.AnyTimes()` for boilerplate calls (`IOStreams`, `Context`, `Config().Authentication()`, `ClientFactory()`).
4. **Return concrete values** that match SDK signatures exactly—pay attention to pointer types (use `types.ToPtr` or take addresses of literals).
5. **Add scenario-specific expectations** in the order the code under test will call them. Use `DoAndReturn` to assert request payloads.
6. **Run the function/command** and assert on outputs (stdout, stderr, returned structs, or errors).
7. **Keep scopes isolated** by creating a fresh controller inside each `t.Run` so expectations never leak between scenarios.

### Minimal `gomock` Example

```go
ctrl := gomock.NewController(t)
t.Cleanup(ctrl.Finish)

mCtx := mocks.NewMockCmdContext(ctrl)
mCfg := mocks.NewMockConfig(ctrl)
mAuth := mocks.NewMockAuthConfig(ctrl)

mCtx.EXPECT().Config().Return(mCfg, nil)
mCfg.EXPECT().Authentication().Return(mAuth).AnyTimes()
mAuth.EXPECT().GetDefaultOrganization().Return("fabrikam", nil)

scope, err := util.ParseScope(mCtx, "")
require.NoError(t, err)
require.Equal(t, "fabrikam", scope.Organization)
require.Empty(t, scope.Project)
```

Remember to import `github.com/tmeckel/azdo-cli/internal/mocks`, and `go.uber.org/mock/gomock` at the top of the test file along with `testing`, `github.com/stretchr/testify/require`, or any other assertion library helpers you leverage.
Use this pattern as a seed and add more mocks only as the code under test demands them. If a method is invoked multiple times, prefer `.AnyTimes()` unless call count matters to the behavior you are asserting.

## Acceptance Tests

Acceptance tests (`*_acc_test.go`) run against a live Azure DevOps organization using the harness under `internal/test`. They are opt-in and should only be used when unit tests and mocks cannot provide enough confidence.

### When to add an acceptance test

- You need to verify a workflow/command (e.g., modifying security permissions) against real data.
- The command interacts with eventual-consistency behaviors that are difficult to emulate in unit tests.
- There is an existing harness entry point (see `internal/cmd/security/permission/delete/delete_acc_test.go`).

### Required environment variables

| Variable           | Purpose                                                                                                |
| ------------------ | ------------------------------------------------------------------------------------------------------ |
| `AZDO_ACC_TEST=1`  | Enables acceptance tests. Without it, `inttest.Test` skips all steps.                                  |
| `AZDO_ACC_ORG`     | Organization name used for the session.                                                                |
| `AZDO_ACC_ORG_URL` | Optional explicit organization URL; defaults to `https://dev.azure.com/<org>`.                         |
| `AZDO_ACC_PAT`     | Personal Access Token with the scopes required by the test steps.                                      |
| `AZDO_ACC_PROJECT` | Project name used by acceptance tests that operate on project-scoped resources.                        |
| `AZDO_ACC_TIMEOUT` | Optional override for the default 60 s timeout. Accepts Go durations (`45s`, `2m`) or integer seconds. |

### Step-by-step skeleton

1. Place the test in the command package with the `_acc_test.go` suffix.
2. Wrap your steps in `inttest.Test(t, inttest.TestCase{ Steps: []inttest.Step{ ... } })`.
3. **PreRun**: create or seed live resources (groups, repositories, permissions) using `ctx.ClientFactory()`.
4. **Run**: construct the command options and call the command’s `run...` helper directly (e.g., `return runCommand(ctx, opts)`).
5. **Verify**: use `inttest.Poll` to wait for eventual consistency and assert desired state.
6. **PostRun**: delete or revert all resources you created; aggregate cleanup errors with `errors.Join`.

Example shell to execute a single acceptance test:
```bash
AZDO_ACC_TEST=1 \
AZDO_ACC_ORG=fabrikam \
AZDO_ACC_PAT=xxxxxxxxxxxxxxxxxxxx \
go test ./internal/cmd/security/permission/delete -run TestAccDeletePermission
```
Acceptance tests are not run in CI; execute them manually before publishing features that depend on live Azure DevOps behavior.

### TestContext helpers & utilities

- `inttest.TestContext` now exposes `Project()` alongside `Org`, `OrgUrl`, and `PAT`. Set `AZDO_ACC_PROJECT` when a test needs to target a specific project and fail fast in `PreRun` if it is missing.
- Use `TestContext.SetValue(key, value)`/`Value(key)` to propagate data across `PreRun`, `Run`, `Verify`, and `PostRun` without relying on package-level variables. Keys can be simple strings or typed aliases; mimic `context.Context` usage.
- The helper `inttest.WriteTestFile(path, contents)` creates or truncates files with `0600` permissions and ensures parent directories exist, which is useful for acceptance tests that need temporary credentials or certificates.

### Updating Mocks

All mocks in this project are generated and managed by a single script.

**To update all existing mocks** after changing an interface, simply run the script:
```bash
./scripts/generate_mocks.sh
```

**To add a new mock** for an interface that is not yet mocked:

1.  **Edit the script:** Open `scripts/generate_mocks.sh` and add a new `mockgen` command for your interface. Follow the existing examples in the script for source files or vendor packages.

    *Example for a local interface:*
    ```bash
    echo "Generating MyInterface mock..."
    mockgen -source internal/path/to/my_interface.go \
      -package=mocks -destination internal/mocks/my_interface_mock.go
    ```

    *Example for a vendored interface:*
    ```bash
    echo "Generating Other client mock..."
    mockgen \
      -package=mocks -destination internal/mocks/other_client_mock.go \
      -mock_names Client=MockOtherClient \
      github.com/some/vendor/package Client
    ```

2.  **Run the script:** Execute `./scripts/generate_mocks.sh` to generate the new mock file in `internal/mocks/`.

3.  **Commit the changes:** Commit both the modified `scripts/generate_mocks.sh` and the new mock file.

This process ensures that all mocks are kept up-to-date and that the generation process is centralized and reproducible. You must follow these steps before attempting to use a new mock in a test.

## Authoring tests (black‑box, spec‑driven)

- Start from the Microsoft Learn documentation for REST v7.1:
  - Example: Pull Requests – Create, Get, Update parameters and payloads.
- Express expectations solely in terms of:
  - CLI flags and inputs
  - External behaviors and error messages
  - REST request arguments: e.g.,
    - `sourceRefName`/`targetRefName` include `refs/heads/`
    - `searchCriteria.status == active` and `top == 1` when checking existing PRs
    - Reviewers list carries descriptors and `isRequired` flags
- Validate by intercepting args in mocks via `DoAndReturn` and asserting fields.
- Avoid coupling to internal helpers unless testing a pure function; prefer testing via the command entry (e.g., `runCmd` with a mocked context).
- Use table‑driven tests where it adds clarity (e.g., sets of negative cases).

### Tips for reliable tests

- Use `io, _, out, errOut := iostreams.Test()` to capture I/O streams. Assert against the string content of the `out` and `errOut` buffers (e.g., `assert.Equal(t, "expected", out.String())`).
- **Ensure Test Data Type Accuracy:** When creating test data structs (e.g., a fake `core.TeamProject`), ensure all field types exactly match the real API struct definitions from the `vendor/` directory. Mismatches (e.g., `core.Time` vs. `azuredevops.Time`) will cause compilation errors.
- Prefer `require` for preconditions and `assert` for value checks.
- Keep the number of expectations minimal; over‑specifying call sequences increases brittleness.
- Assert error text that the UX guarantees (don’t overfit on punctuation or variable content).
- Prevent import cycles when using generated mocks. Every file in `internal/mocks` imports the package that defines the interface being mocked. If your test lives inside that package and imports its mock, Go observes `package → internal/mocks → package` and fails with `import cycle not allowed`.
  - **How to check**: search `scripts/generate_mocks.sh` or `internal/mocks/*_mock.go` for the interface name. If your interface (or function you are testing) appears there, the mocks will re-import the package.
  - **If you only need exported behaviour**: write the test in an **external** package by adding `_test` to the package name (for example `package extensions_test`). Import the package under test with an alias (`extensions "github.com/tmeckel/azdo-cli/internal/azdo/extensions"`) and use its exported symbols. The dependency chain becomes `extensions_test → internal/mocks → internal/azdo/...`, so no cycle occurs.
  - **If you must reach unexported helpers**: keep the test in the original package but avoid importing the generated mock. Instead, create lightweight local fakes/stubs in the test file, or refactor the code so that the behaviour can be exercised through exported seams.
  - **Example**: `internal/azdo/extensions` declares interfaces that are mocked in `internal/mocks/extension_mock.go`. A unit test for `extensionClient` that imports `mocks.NewMockConnection` must use `package extensions_test`; otherwise Go reports an import cycle because both the test and the mock pull in `internal/azdo/extensions`.

## Coverage

- Run with coverage for a package:
  - `go test -count=1 -cover -coverprofile=cover.out ./internal/cmd/pr/create`
- View overall function/file coverage:
  - `go tool cover -func=cover.out`
  - Filter for a specific file:
    - `go tool cover -func=cover.out | rg internal/cmd/pr/create/create.go`
- Open annotated HTML:
  - `go tool cover -html=cover.out -o coverage.html`
- Include only a specific package in coverage accounting:
  - `go test -count=1 -cover -coverprofile=cover.out -coverpkg=github.com/tmeckel/azdo-cli/internal/cmd/pr/create ./internal/cmd/pr/create`
- For concurrency‑safe accounting:
  - `go test -count=1 -covermode=atomic -coverprofile=cover.out ./...`

## Running tests

- All tests: `go test ./...`
- With timeout: `TIMEOUT=60s go test ./...`
- Single package: `go test ./internal/cmd/pr/create`
- By name: `go test ./internal/cmd/pr/create -run '^TestPullRequest_'`

### Formatting before you commit

- Format touched Go files (if available): `gofumpt -w <files>`
- Organize imports (if available): `goimports -w <files>`
- Regenerate mocks after interface changes: `./scripts/generate_mocks.sh`
- Re-run the smallest relevant `go test ./internal/...` command before `go test ./...` to tighten the feedback loop.

## What to avoid

- When designing unit tests: **no network calls**; do not instantiate real Azure DevOps connections.
- No reliance on local git state; always go through `GitCommand` mocks.
- Do not lock tests to internal implementation details that can change without affecting behavior.

## When adding new commands

- Derive expected REST interactions from the docs.
- Decide what to unit test (args, request shapes, error handling) vs. what belongs in e2e/smoke tests.
- Add tests for edge cases (nil fields in models, missing defaults, empty lists) to ensure robust handling.

---
By following these guidelines, tests stay focused on external behavior, are robust to refactors, and provide reliable safety against regressions while accurately reflecting the Azure DevOps REST API v7.1 contract.

### Additional Recommendations from Test Analysis

- **Do not duplicate production logic in tests**: Always invoke the same parsing or validation helpers used in non‑test code instead of re‑implementing them inline. This avoids divergence between test behavior and actual command logic.
- **Use table‑driven subtests for related scenarios**: Group success/error paths that share setup into a single table‑driven test with subtests (`t.Run`) for clarity and ease of extension.
- **Verify API argument shapes against specifications**: When mocking Azure DevOps client calls, explicitly validate fields in argument structs (`CreateRepositoryArgs`, etc.) against REST API v7.1 documentation, not just accept them blindly.
- **Avoid non‑essential expectations**: Do not set `.EXPECT()` calls for printers, connection factories, or client factories unless those interactions are central to the scenario outcome; use `.AnyTimes()` only for incidental calls.
- **Abstract common mock setups**: Create helper functions in test packages to configure frequently‑used mocks (`CmdContext`, printer, connection/client factories) to reduce boilerplate and ensure consistent expectations.
- **Cover edge‑case returns**: Add scenarios where mocked API clients return objects with nil/empty fields to validate CLI robustness against incomplete or partial data in responses.

## Mocking Guidelines and Common Pitfalls

From recent issues encountered in `internal/cmd/repo/create/create_test.go`, the following errors and fixes emerged:

**Typical Errors:**
- **Missing Expectations**: Tests failed with "Unexpected call" because `CmdContext.Config()`, `CmdContext.Context()`, `CmdContext.Printer()`, `ClientFactory.Git()` and `ConnectionFactory.Connection()` were not mocked when `runCreate` invoked them.
- **Incorrect Mock Types**: Fake structs used instead of generated mocks for `Config` and `AliasConfig` led to interface implementation errors.
- **Signature Mismatches**: `CreateRepository` mocks returned values instead of pointers, or wrong argument types, not matching `func(context.Context, git.CreateRepositoryArgs) (*git.GitRepository, error)`.
- **Nil Pointer Dereference**: Not returning `nil` correctly from mocked methods like `Printer.AddField()` caused panics.
- **Over‑Mocking**: Expectations set for calls that did not happen in certain paths, causing brittleness.
- **Pointer vs Value Issues**: Mismatches with pointer usage in API client returns caused compile/runtime failures.

**Fixes Applied:**
- Added all necessary mock expectations for methods actually invoked in each test scenario.
- Used generated mocks from `internal/mocks` for `Config`, `AliasConfig`, and `AuthConfig` rather than fakes.
- Corrected function signatures in mocks to exactly match implementations, including pointer return types.
- Returned `nil` appropriately from mocked methods to match usage.
- Limited expectations to only relevant calls per scenario and used `.AnyTimes()` for repeated/non‑critical calls.
- Verified argument types against actual definitions before setting up mocks.

**Conclusions to Prevent Future Mistakes:**
- **Review code under test before mocking** to know exactly which calls need expectations. Do not assume a function will fail early; if it proceeds, subsequent calls will need to be expected by your mocks, otherwise `gomock` will report an 'unexpected call' error.
- **Always use generated mocks** for interfaces unless replacements fully match signatures.
- **Match method signatures exactly**, minding pointer/value distinctions.
- **Return appropriate values** to avoid runtime exceptions.
- **Avoid over‑mocking**; focus on what’s directly needed for test validation.
- **Use `.AnyTimes()` liberally** in cases where call frequency isn’t central to the test outcome.
- **Check argument matchers** to ensure expected and actual calls align.

### Scenario Isolation in Tests

When using `gomock`, each scenario that requires different mock configurations or expectations must be run in a fully isolated context:
- **Separate test functions or subtests**: Use distinct `TestXxx` functions or `t.Run` blocks for each logical scenario. Subtests should create a new `gomock.Controller` and new mocks internally.
- **Prefer `t.Run`** for related scenarios: This keeps them grouped under one top-level test while still providing isolation, avoiding excessive proliferation of separate test functions.
- **Avoid state leakage**: Reset mocks between scenarios to prevent expectations from previous runs affecting subsequent ones.
- **Scoped expectations**: Keep expectations minimal and relevant to the scenario, ensuring no unused calls remain configured.
- **Fresh controller per scenario**: This guarantees that call counts and ordering validations remain accurate and independent.

Mixing scenarios with different dependency interactions into a single test is discouraged—it leads to brittle setups and unexpected call errors when calls occur outside the configured expectations.

## Self-Contained Test Setup

To ensure test clarity and make the codebase easier to parse for all contributors (including AI agents), each test function should be self-contained. This means all mock objects and their baseline expectations should be defined inline within the test function itself (`TestXxx` or a `t.Run` block).

While this approach may lead to some code duplication, it makes each test case explicit and easy to understand without needing to reference external helper functions for setup.

### Standard Mocking Procedure for a Command Test

For any given command test, follow these steps inside your `Test...` function or `t.Run` block:

1. **Initialize `gomock` Controller:** Start by creating the controller and deferring its `Finish` call.
    ```go
    ctrl := gomock.NewController(t)
    t.Cleanup(ctrl.Finish) // Or defer ctrl.Finish() for older Go versions
    ```

2. **Instantiate Mocks:** Create instances of all required mocks. The most common set includes `CmdContext` and its dependencies.
    ```go
    io, _, _, _ := iostreams.Test()
    mCmdCtx := mocks.NewMockCmdContext(ctrl)
    mRepoCtx := mocks.NewMockRepoContext(ctrl)
    mRepo := mocks.NewMockRepository(ctrl)
    mGitClient := mocks.NewMockAzDOGitClient(ctrl)
    mClientFactory := mocks.NewMockClientFactory(ctrl)
    mConfig := mocks.NewMockConfig(ctrl)
    mAuth := mocks.NewMockAuthConfig(ctrl)
    ```

3. **Set Baseline Expectations:** Set the minimum required expectations for the command to run without errors from the framework itself. These are typically calls to `CmdContext` that provide other objects like `IOStreams`, `Config`, etc. Use `.AnyTimes()` for these calls as they are not the focus of the test.
    ```go
    // Core CmdContext expectations
    mCmdCtx.EXPECT().IOStreams().Return(io, nil).AnyTimes()
    mCmdCtx.EXPECT().Context().Return(context.Background()).AnyTimes()
    mCmdCtx.EXPECT().RepoContext().Return(mRepoCtx).AnyTimes()
    mCmdCtx.EXPECT().ClientFactory().Return(mClientFactory).AnyTimes()
    mCmdCtx.EXPECT().Config().Return(mConfig, nil).AnyTimes()

    // Core RepoContext expectation
    mRepoCtx.EXPECT().Repo().Return(mRepo, nil).AnyTimes()

    // Core Config expectation
    mConfig.EXPECT().Authentication().Return(mAuth).AnyTimes()
    ```

4. **Set Scenario-Specific Expectations:** After the baseline is established, set the specific expectations for the behavior you are testing.
    ```go
    // Scenario: Test that the command correctly fetches a PR by ID.
    mRepo.EXPECT().GitClient(gomock.Any(), gomock.Any()).Return(mGitClient, nil)
    mGitClient.EXPECT().GetPullRequestById(gomock.Any(), gomock.Any()).Return(&git.GitPullRequest{
        PullRequestId: types.ToPtr(123),
    }, nil)
    ```

5. **Execute and Assert:** Run the command and assert the outcome.

By following this pattern, each test explicitly declares all its dependencies and their expected behaviors, making the test easy to read, understand, and maintain.
