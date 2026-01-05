# Service Endpoint Framework (`internal/cmd/serviceendpoint`)

This directory contains the CLI command tree for working with Azure DevOps service endpoints (service connections) and the shared “typed endpoint” framework used by per-type create commands (e.g. `github`, `azurerm`).

The framework’s goal is to make it easy to add **new typed service endpoint commands** while keeping behavior consistent (scope parsing, progress indicator, optional validation, readiness wait, connection test, pipeline permissions, output formatting).

## Command overview

- Top-level group: `azdo service-endpoint …` (`internal/cmd/serviceendpoint/serviceendpoint.go`)
- Generic create/import (JSON payload): `azdo service-endpoint create [ORG/]PROJECT --from-file …` (`internal/cmd/serviceendpoint/create/create.go`)
- Typed create commands: `azdo service-endpoint create <type> [ORG/]PROJECT …` (subcommands under `internal/cmd/serviceendpoint/create/`)
- Update: `azdo service-endpoint update [ORG/]PROJECT/ID_OR_NAME …` (`internal/cmd/serviceendpoint/update/update.go`)
- Show/List/Delete/Export: `internal/cmd/serviceendpoint/show`, `list`, `delete`, `export`

## Shared framework components

All shared primitives live under `internal/cmd/serviceendpoint/shared/`.

### Typed create runner

The runner is implemented in `internal/cmd/serviceendpoint/shared/runner_create.go`:

- Entry point: `shared.RunTypedCreate(cmd, args, cfg)`
- Responsibilities:
  - parse scope from the positional argument (`[ORG/]PROJECT`)
  - resolve the target project reference
  - build a common `serviceendpoint.ServiceEndpoint` skeleton (name/description/type/owner/project refs)
  - call the type-specific configurer to populate:
    - `endpoint.Url`
    - `endpoint.Authorization` (scheme + parameters)
    - `endpoint.Data`
  - optional behaviors driven by common flags:
    - `--validate-schema`
    - `--wait`
    - `--validate-connection`
    - `--grant-permission-to-all-pipelines`
  - redact authorization parameters before output
  - output (JSON export when requested, otherwise template output)

### Common typed-create flags

Typed create commands register common flags via `shared.AddCreateCommonFlags(cmd)` in `internal/cmd/serviceendpoint/shared/create_common.go`.

These flags are shared across all typed create commands:

- `--name` (required)
- `--description`
- `--validate-schema`
- `--wait`
- `--timeout`
- `--validate-connection`
- `--grant-permission-to-all-pipelines`
- JSON output options via `util.AddJSONFlags` (`--json`, `--jq`, `--template`)

Implementation detail: `AddCreateCommonFlags` stores a `createCommonOptions` value in the Cobra command context under the key `createCommonOptions`. The typed runner reads those options from `cmd.Context()`.

### Metadata validation (`--validate-schema`)

When `--validate-schema` is set, `shared.RunTypedCreate` calls:

- `shared.ValidateEndpointAgainstMetadata` (`internal/cmd/serviceendpoint/shared/type_validate.go`)

That validator fetches live endpoint type metadata via:

- `shared.GetServiceEndpointTypes` (`internal/cmd/serviceendpoint/shared/type_registry.go`)

Validation is currently focused on **authorization schema correctness**:

- endpoint type exists
- authorization scheme exists for that type
- required authorization parameter keys (from `inputDescriptors[].validation.isRequired`) are present

Gotchas:

- This validation requires fetching live endpoint type metadata from the organization. If the metadata request fails (permissions/network/organization settings), `--validate-schema` will fail the command.
- Validation is limited to auth scheme/parameter presence; it does not validate endpoint URLs, reachability, or correctness of non-auth `Data` fields.

### Readiness wait (`--wait`)

Readiness wait is implemented in `internal/cmd/serviceendpoint/shared/wait_ready.go` and uses `internal/util/poll.go`.

The runner polls `GetServiceEndpointDetails` until:

- the endpoint reports `IsReady == true`, or
- the endpoint reports a terminal failure (`operationStatus.state == "failed"` when present)

### TestConnection (`--validate-connection`)

Connection validation is implemented in `internal/cmd/serviceendpoint/shared/test_connection.go`.

Behavior:

- fetch metadata and verify the endpoint type supports a `TestConnection` data source
- execute/poll `ExecuteServiceEndpointRequest` until the result’s `StatusCode` becomes `"ok"` (case-insensitive) or the timeout is reached

Gotchas:

- Not every service endpoint type supports `TestConnection`; in that case the command fails with an explicit “not supported” error when `--validate-connection` is enabled.
- This uses the organization’s endpoint type metadata to find the `TestConnection` data source, so it can fail for the same reasons as `--validate-schema` (metadata fetch issues).

### Pipeline permissions (`--grant-permission-to-all-pipelines`)

Pipeline permission granting is implemented in `internal/cmd/serviceendpoint/shared/pipeline_permissions.go` using the `pipelinepermissions` client.

In the typed create runner, this step runs after creation (and after wait/test if enabled). If granting fails, the runner attempts rollback by deleting the created endpoint.

For typed update commands, permission changes are only applied when the flag is explicitly provided. To revoke access for all pipelines, pass an explicit false value: `--grant-permission-to-all-pipelines=false`.

### Output and redaction

Output rendering is centralized in `internal/cmd/serviceendpoint/shared/output.go` and `internal/cmd/serviceendpoint/shared/show.tpl`.

Typed create redaction:

- `shared.RunTypedCreate` calls `shared.RedactSecrets(created)` (`internal/cmd/serviceendpoint/shared/endpoints.go`) before output.
- Current behavior is intentionally conservative: all authorization parameter values are replaced with `"REDACTED"`.

Note: other commands (e.g. `show`, `update`) currently call `shared.Output` directly. If an API response ever contains sensitive authorization parameters, template output will display them unless the caller redacts first. Typed create already does this.

Note: the typed update runner also redacts before output (`shared.RunTypedUpdate`), but the non-typed `service-endpoint update` command does not currently redact before calling `shared.Output`.

Implementation detail: the default template output (`shared/show.tpl`) prints `Authorization.Parameters` when present, so redaction needs to happen before calling `shared.Output`.

## How to add a new typed create command

Use existing commands as references:

- GitHub: `internal/cmd/serviceendpoint/create/github/create.go`
- AzureRM: `internal/cmd/serviceendpoint/create/azurerm/create.go`

### 1) Create the new package

Create a new directory:

`internal/cmd/serviceendpoint/create/<type>/`

Then implement `create.go` with a factory:

- `func NewCmd(ctx util.CmdContext) *cobra.Command`

### 2) Implement the configurer

Typed create commands use a small interface (see `internal/cmd/serviceendpoint/shared/runner_create.go`):

```go
type EndpointTypeConfigurer interface {
  CommandContext() util.CmdContext
  TypeName() string
  Configure(endpoint *serviceendpoint.ServiceEndpoint) error
}
```

Recommended structure:

- define a `*<type>Configurer` struct
  - embed/contain `cmdCtx util.CmdContext`
  - add fields for type-specific flags
- implement:
  - `CommandContext()` returning the injected context
  - `TypeName()` returning the Azure DevOps endpoint type identifier (e.g. `github`, `azurerm`)
  - `Configure(endpoint)` populating `Url`, `Authorization`, and `Data`

### 3) Wire Cobra flags and the shared runner

In `NewCmd`:

1) instantiate `cfg := &<type>Configurer{cmdCtx: ctx}`
2) create a Cobra command with `Args: cobra.ExactArgs(1)` where the arg is `[ORG/]PROJECT`
3) bind type-specific flags onto `cfg` fields
4) call `shared.AddCreateCommonFlags(cmd)` to add common framework flags (this is required; the shared runner reads options from `cmd.Context()` and will panic if the value is missing)
5) set `RunE` to `return shared.RunTypedCreate(cmd, args, cfg)`

### 4) Register the subcommand

In `internal/cmd/serviceendpoint/create/create.go`, add:

```go
cmd.AddCommand(<type>.NewCmd(ctx))
```

### 5) Tests

Typed create commands are tested via their **public Cobra surface**:

- Build the command with `NewCmd(ctx)`
- Provide args/flags with `cmd.SetArgs(...)`
- Execute with `cmd.Execute()`

This is required because type-specific logic now lives in the configurer + shared runner (`shared.RunTypedCreate`), not in a package-local `runCreate` helper.

#### Unit tests (hermetic, preferred)

Unit tests should be table-driven and mock the command context and clients that the shared runner uses.

Minimum mocks for a typed create test:

- `CmdContext.Context()`, `CmdContext.IOStreams()`
- `CmdContext.ClientFactory()`
- `core.Client.GetProject(...)` (used by `shared.ResolveProjectReference`)
- `serviceendpoint.Client.CreateServiceEndpoint(...)`

Mocks when common flags are enabled:

- `--validate-schema`: `serviceendpoint.Client.GetServiceEndpointTypes(...)`
- `--wait`: `serviceendpoint.Client.GetServiceEndpointDetails(...)`
- `--validate-connection`: `serviceendpoint.Client.ExecuteServiceEndpointRequest(...)` (and related polling calls)
- `--grant-permission-to-all-pipelines`: `pipelinepermissions.Client.UpdatePipelinePermisionsForResource(...)` (and `serviceendpoint.Client.DeleteServiceEndpoint(...)` for rollback paths)

Assertions should focus on:

- The endpoint payload passed to `CreateServiceEndpoint` (type, URL, auth scheme, expected auth/data keys)
- Optional follow-up calls are only made when the corresponding flags are set

Reference implementation:

- `internal/cmd/serviceendpoint/create/github/create_test.go`

#### Acceptance tests (live Azure DevOps)

Acceptance tests should keep the existing harness (`internal/test`) and only change **how the command is invoked**:

- In the test step `Run`, construct and execute the command:
  - `cmd := <type>.NewCmd(ctx)`
  - `cmd.SetArgs([]string{projectArg, "--name", ..., <type flags>...})`
  - `return cmd.Execute()`
- In `Verify`, poll for eventual consistency using `internal/util/poll.go` and assert stable fields.
- In `PostRun`, clean up created endpoints (see helpers under `internal/cmd/serviceendpoint/test`).

Environment variables / gating are handled by the acceptance harness in `internal/test/helpers.go` (e.g. `AZDO_ACC_TEST`, `AZDO_ACC_ORG`, `AZDO_ACC_PAT`, `AZDO_ACC_PROJECT`).

Examples:

- `internal/cmd/serviceendpoint/create/azurerm/create_acc_test.go`
- `internal/cmd/serviceendpoint/create/github/create_acc_test.go`

### Typed update runner

The runner is implemented in `internal/cmd/serviceendpoint/shared/runner_update.go`:

- Entry point: `shared.RunTypedUpdate(cmd, args, cfg)`
- Responsibilities:
  - parse scope from the positional argument (`[ORG/]PROJECT/ID_OR_NAME`)
  - resolve the existing endpoint
  - apply common field updates (name, description)
  - call the type-specific configurer to update:
    - `endpoint.Url`
    - `endpoint.Authorization`
    - `endpoint.Data`
  - optional behaviors (validate schema, update pipeline permissions, etc.)
  - redact secrets and output

### Common typed-update flags

Typed update commands register common flags via `shared.AddUpdateCommonFlags(cmd)` in `internal/cmd/serviceendpoint/shared/update_common.go`.

These flags are shared across all typed update commands:

- `--name` (optional)
- `--description` (optional)
- `--wait`
- `--timeout`
- `--validate-schema`
- `--validate-connection`
- `--grant-permission-to-all-pipelines`
- JSON output options

## How to add a new typed update command

This follows the same pattern as typed create.

### 1) Create the new package

Create a new directory:

`internal/cmd/serviceendpoint/update/<type>/`

Then implement `update.go` with a factory:

- `func NewCmd(ctx util.CmdContext) *cobra.Command`

### 2) Implement the configurer

Reuse or create an `EndpointTypeConfigurer` (same interface as create). The `Configure` method will be called with the *existing* endpoint, allowing you to modify fields based on flags.

### 3) Wire Cobra flags and the shared runner

In `NewCmd`:

1) instantiate `cfg := &<type>Configurer{cmdCtx: ctx}`
2) create a Cobra command with `Args: cobra.ExactArgs(1)`
3) bind type-specific flags onto `cfg` fields
4) call `shared.AddUpdateCommonFlags(cmd)` (this is required; the shared runner reads options from `cmd.Context()` and will panic if the value is missing)
5) set `RunE` to `return shared.RunTypedUpdate(cmd, args, cfg)`

### 4) Register the subcommand

In `internal/cmd/serviceendpoint/update/update.go` (or wherever the parent command is), add:

```go
cmd.AddCommand(<type>.NewCmd(ctx))
```
