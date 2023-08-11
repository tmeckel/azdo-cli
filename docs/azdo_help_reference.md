## azdo reference
# azdo reference

## `azdo auth <command>`

Authenticate azdo and git with Azure DevOps

### `azdo auth login [flags]`

Authenticate with a Azure DevOps oragnization

```
-p, --git-protocol string      The protocol to use for git operations: {ssh|https}
    --insecure-storage         Save authentication credentials in plain text instead of credential store
-o, --organizationUrl string   The URL to the Azure DevOps organization to authenticate with
    --with-token               Read token from standard input
````

### `azdo auth logout [flags]`

Log out of a Azure DevOps organization

```
-o, --organization string   The Azure DevOps organization to log out of
````

### `azdo auth setup-git [flags]`

Setup git with AzDO CLI

```
-o, --organization string   Configure git credential helper for specific organization
````

### `azdo auth status [flags]`

View authentication status

```
-o, --organization string   Check a specific oragnizations's auth status
````

## `azdo config <command>`

Manage configuration for azdo

### `azdo config get <key> [flags]`

Print the value of a given configuration key

```
-o, --organization string   Get per-organization setting
````

### `azdo config list [flags]`

Print a list of configuration keys and values

```
-o, --organization string   Get per-organization configuration
````

### `azdo config set <key> <value> [flags]`

Update configuration with a value for the given key

```
-o, --organization string   Set per-organization setting
````

## `azdo project <command> [flags]`

Work with Azure DevOps Projects.

### `azdo project list [flags]`

List the projects for an organization

```
    --format string         Output format: {json}
-l, --limit int             Maximum number of projects to fetch (default 30)
-o, --organization string   Get per-organization configuration
    --state string          Project state filter: {deleting|new|wellFormed|createPending|all|unchanged|deleted}
````

## `azdo repo <command>`

Manage repositories

### `azdo repo clone <repository> [<directory>] [-- <gitflags>...]`

Clone a repository locally

```
    --no-credential-helper          Don't configure azdo as credential helper for the cloned repository
-o, --organization string           Use organization
-p, --project string                Use project
-u, --upstream-remote-name string   Upstream remote name when cloning a fork (default "upstream")
````

### `azdo repo list <project> [flags]`

List repositories of a project inside an organization

```
    --format string         Output format: {json}
    --include-hidden        Include hidden repositories
-L, --limit int             Maximum number of repositories to list (default 30)
-o, --organization string   Get per-organization configuration
    --visibility string     Filter by repository visibility: {public|private}
````


### See also

* [azdo](./azdo.md)
