## Command `azdo reference`

# azdo reference

## `azdo auth <command>`

Authenticate azdo and git with Azure DevOps

### `azdo auth login [flags]`

Authenticate with a Azure DevOps organization

```
-p, --git-protocol string       The protocol to use for git operations: {ssh|https}
    --insecure-storage          Save authentication credentials in plain text instead of credential store
-o, --organization-url string   The URL to the Azure DevOps organization to authenticate with
    --with-token                Read token from standard input
```

### `azdo auth logout [ORG]`

Log out of a Azure DevOps organization

### `azdo auth setup-git [flags]`

Setup git with AzDO CLI

```
-o, --organization string   Configure git credential helper for specific organization
```

### `azdo auth status [organization]`

View authentication status

## `azdo boards <command>`

Work with Azure Boards resources.

Aliases

```
b
```

### `azdo boards area <command>`

Manage area paths used by Azure Boards.

Aliases

```
a
```

#### `azdo boards area project <command>`

Manage area paths scoped to a project.

Aliases

```
prj, p
```

##### `azdo boards area project list [ORGANIZATION/]PROJECT [flags]`

List area paths defined for a project.

```
    --depth int         Depth of child nodes to include (use 0 to omit child nodes). (default 1)
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --path string       Restrict results to a specific area path (relative paths like "Payments" or "Payments/Sub" are resolved under <project>/Area).
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
ls, l
```

### `azdo boards iteration <command>`

Work with iteration/classification nodes.

Aliases

```
it, i
```

#### `azdo boards iteration project <command>`

Project-scoped iteration commands.

Aliases

```
prj, p
```

##### `azdo boards iteration project list [ORGANIZATION/]PROJECT [flags]`

List iteration hierarchy for a project.

```
-d, --depth int            Depth to fetch (1-10) (default 3)
    --finish-date string   Apply a comparison filter to iteration finish dates; supports operators like <= and special value "today" (e.g., "<=today")
    --include-dates        Include iteration start and finish dates
-q, --jq expression        Filter JSON output using a jq expression
    --json fields[=*]      Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-p, --path string          Iteration path relative to project root
    --start-date string    Apply a comparison filter to iteration start dates; supports operators like >= and special value "today" (e.g., ">=today")
-t, --template string      Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
ls, l
```

## `azdo co`

Alias for "pr checkout"

## `azdo config <command>`

Manage configuration for azdo

### `azdo config get <key> [flags]`

Print the value of a given configuration key

```
-o, --organization string   Get per-organization setting
```

### `azdo config list [flags]`

Print a list of configuration keys and values

```
    --all                   Show config options which are not configured
-o, --organization string   Get per-organization configuration
```

Aliases

```
ls
```

### `azdo config set <key> <value> [flags]`

Update configuration with a value for the given key

```
-o, --organization string   Set per-organization setting
-r, --remove                Remove config item for an organization, so that the default value will be in effect again
```

## `azdo graph <command>`

Manage Azure DevOps Graph resources (users, groups)

### `azdo graph user <command>`

Manage users in Azure DevOps

#### `azdo graph user list [project] [flags]`

List users and groups in Azure DevOps

```
-F, --filter string         Filter users by prefix (max 100 results)
-q, --jq expression         Filter JSON output using a jq expression
    --json fields[=*]       Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-L, --limit int             Maximum number of users to return (pagination client-side) (default 20)
-o, --organization string   Organization name. If not specified, defaults to the default organization
-t, --template string       Format JSON output using a Go template; see "azdo help formatting"
-T, --type strings          Subject types filter (comma-separated). If not specified defaults to 'aad'
```

Aliases

```
ls
```

## `azdo pipelines`

Manage Azure DevOps pipelines

Aliases

```
p
```

### `azdo pipelines variable-group`

Manage Azure DevOps variable groups

Aliases

```
variablegroup, variable-groups, variablegroups, vg
```

#### `azdo pipelines variable-group create [ORGANIZATION/]PROJECT/NAME [flags]`

Create a variable group

```
-A, --authorize                          Authorize the variable group for all pipelines in the project after creation
-d, --description string                 Optional description for the variable group
-q, --jq expression                      Filter JSON output using a jq expression
    --json fields[=*]                    Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --keyvault-name string               Azure Key Vault name backing the variable group
    --keyvault-secret strings            Map a pipeline variable to a Key Vault secret (variable=secretName); repeat for multiple entries
    --keyvault-service-endpoint string   Service endpoint ID (UUID) or name, that grants access to the Azure Key Vault
    --project-reference strings          Additional project names or IDs to share the group with (repeat or comma-separate)
    --provider-data-json string          Raw JSON payload for providerData (advanced; cannot be combined with Key Vault options)
    --secret strings                     Seed secret variables using key[=value]; value falls back to AZDO_PIPELINES_SECRET_<NAME> or an interactive prompt
-t, --template string                    Format JSON output using a Go template; see "azdo help formatting"
    --type string                        Variable group type: {Vsts|AzureKeyVault} (default "Vsts")
-v, --variable strings                   Seed non-secret variables using key=value[;readOnly=true|false]
```

#### `azdo pipelines variable-group delete [ORGANIZATION/]PROJECT/GROUP [flags]`

Delete a variable group from a project

```
    --all                         Remove the variable group from every assigned project
-q, --jq expression               Filter JSON output using a jq expression
    --json fields[=*]             Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --project-reference strings   Additional project names or IDs to remove the group from (repeatable, comma-separated)
-t, --template string             Format JSON output using a Go template; see "azdo help formatting"
-y, --yes                         Skip the confirmation prompt.
```

Aliases

```
rm, del, d
```

#### `azdo pipelines variable-group list [ORGANIZATION/]PROJECT [flags]`

List variable groups

```
    --action string     Action filter string (e.g., 'manage', 'use'): {none|manage|use}
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --max-items int     Optional client-side cap on results; stop fetching once reached
    --order string      Order of variable groups (asc, desc): {desc|asc} (default "desc")
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
    --top int           Server-side page size hint (positive integer)
```

Aliases

```
ls, l
```

#### `azdo pipelines variable-group show [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME [flags]`

Show variable group details

```
    --include-project-references   Include variableGroupProjectReferences details
    --include-provider-data        Include providerData payloads
    --include-variables            Include variable values (secrets remain redacted)
-q, --jq expression                Filter JSON output using a jq expression
    --json fields[=*]              Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-t, --template string              Format JSON output using a Go template; see "azdo help formatting"
```

#### `azdo pipelines variable-group variable`

Manage variables in a variable group

Aliases

```
var
```

##### `azdo pipelines variable-group variable list [ORGANIZATION/]PROJECT/VARIABLEGROUP [flags]`

List variables in a variable group

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
```

## `azdo pr <command> [flags]`

Manage pull requests

```
--help   Show help for command
```

### `azdo pr checkout <number> [flags]`

Check out a pull request in git

```
-b, --branch string        Local branch name to use (default [the name of the head branch])
    --detach               Checkout PR with a detached HEAD
-f, --force                Reset the existing local branch to the latest state of the pull request
    --recurse-submodules   Update all submodules after checkout
```

### `azdo pr close <number> | <branch> | <url> [flags]`

Close a pull request

```
-c, --comment string   Leave a closing comment
-d, --delete-branch    Delete the local and remote branch after close
```

### `azdo pr comment [<number> | <branch> | <url>] [flags]`

Comment a pull request

```
-c, --comment string   Comment to add to the pull request. Use '-' to read from stdin.
-t, --thread int       ID of the thread to reply to.
```

### `azdo pr create [flags]`

Create a pull request

```
-B, --base branch                 The branch into which you want your code merged
-D, --description string          Description for the pull request
-F, --description-file file       Read description text from file (use "-" to read from standard input)
-d, --draft                       Mark pull request as a draft
    --dry-run                     Print details instead of creating the PR. May still push git changes.
-f, --fill                        Use commit info for title and body
    --fill-first                  Use first commit info for title and body
    --fill-verbose                Use commits msg+body for description
-H, --head branch                 The branch that contains commits for your pull request (default [current branch])
-o, --optional-reviewer strings   Optional reviewers (comma-separated)
    --recover string              Recover input from a failed run of create
-r, --required-reviewer strings   Required reviewers (comma-separated)
-t, --title string                Title for the pull request
    --use-template                Use a pull request template for the description of the new pull request. The command will fail if no template is found
```

Aliases

```
new
```

### `azdo pr diff [<number> | <branch> | <url>] [flags]`

View changes in a pull request

```
--color string   Use color in diff output: {always|never|auto} (default "auto")
--name-only      Display only names of changed files
```

### `azdo pr edit [<number> | <branch> | <url>] [flags]`

Edit a pull request

```
    --add-label strings               Add labels (comma-separated)
    --add-optional-reviewer strings   Add or demote optional reviewers (comma-separated)
    --add-required-reviewer strings   Add or promote required reviewers (comma-separated)
-B, --base string                     Change the base branch for this pull request
-b, --body string                     Set the new body.
-F, --body-file string                Read body text from file (use "-" to read from standard input)
    --remove-label strings            Remove labels (comma-separated, use * to remove all)
    --remove-reviewer strings         Remove reviewers (comma-separated, use * to remove all)
-t, --title string                    Set the new title.
```

### `azdo pr list [[organization/]project/repository] [flags]`

List pull requests in a repository or a project

```
-a, --author string       Filter by author
-B, --base string         Filter by base branch
-d, --draft               Filter by draft state
-H, --head string         Filter by head branch
-q, --jq expression       Filter JSON output using a jq expression
    --json fields[=*]     Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-l, --label strings       Filter by label
-L, --limit int           Maximum number of items to fetch (default 30)
-m, --mergestate string   Filter by merge state: {succeeded|conflicts}
-r, --reviewer string     Filter by reviewer
-s, --state string        Filter by state: {abandoned|active|all|completed} (default "active")
-t, --template string     Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
ls
```

### `azdo pr merge <number> | <branch> | <url> [flags]`

Merge a pull request

```
-d, --delete-source-branch    Delete the source branch after merging
    --merge-strategy string   Merge strategy to use: {noFastForward|squash|rebase|rebaseMerge} (default "NoFastForward")
-m, --message string          Message to include when completing the pull request
    --transition-work-items   Transition linked work item statuses upon merging (default true)
```

### `azdo pr reopen <number> | <branch> | <url> [flags]`

Reopen a pull request

```
-c, --comment string   Add a reopening comment
```

### `azdo pr view [<number> | <branch> | <url>] [flags]`

View a pull request

```
    --comment-sort string   Sort comments by creation time; defaults to 'desc' (newest first): {desc|asc} (default "desc")
    --comment-type string   Filter comments by type; defaults to 'text': {text|system|all} (default "text")
-c, --comments              View pull request comments
-C, --commits               View pull request commits
-q, --jq expression         Filter JSON output using a jq expression
    --json fields[=*]       Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-r, --raw                   View pull request raw
-t, --template string       Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
show, status
```

### `azdo pr vote [<number> | <branch> | <url>] [flags]`

Vote on a pull request

```
--vote string   Vote value to set: {approve|approve-with-suggestions|reject|reset|wait-for-author} (default "approve")
```

## `azdo project <command> [flags]`

Work with Azure DevOps Projects.

Aliases

```
p
```

### `azdo project create [ORGANIZATION/]PROJECT [flags]`

Create a new Azure DevOps Project

```
-d, --description string      Description for the new project
-q, --jq expression           Filter JSON output using a jq expression
    --json fields[=*]         Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --max-wait int            Maximum wait time in seconds (default 3600)
    --no-wait                 Do not wait for the project to be created
-p, --process string          Process to use (e.g., Scrum, Agile, CMMI) (default "Agile")
-s, --source-control string   Source control type (git or tfvc) (default "git")
-t, --template string         Format JSON output using a Go template; see "azdo help formatting"
    --visibility string       Project visibility (private or public) (default "private")
```

Aliases

```
cr, c, new, n, add, a
```

### `azdo project delete [ORGANIZATION/]PROJECT [flags]`

Delete a project

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --max-wait int      Maximum wait time in seconds (default 3600)
    --no-wait           Do not wait for the project deletion to complete
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
-y, --yes               Skip confirmation prompt
```

Aliases

```
d
```

### `azdo project list [organization] [flags]`

List the projects for an organization

```
    --format string   Output format: {json} (default "table")
-l, --limit int       Maximum number of projects to fetch (default 30)
    --state string    Project state filter: {deleting|new|wellFormed|createPending|all|unchanged|deleted}
```

Aliases

```
ls, l
```

### `azdo project show [ORGANIZATION/]PROJECT [flags]`

Show details of an Azure DevOps Project

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
s
```

## `azdo repo <command>`

Manage repositories

Aliases

```
r
```

### `azdo repo clone [organization/]project/repository [<directory>] [-- <gitflags>...]`

Clone a repository locally

```
    --no-credential-helper          Don't configure azdo as credential helper for the cloned repository
    --recurse-submodules            Update all submodules after checkout
-u, --upstream-remote-name string   Upstream remote name when cloning a fork (default "upstream")
```

Aliases

```
c
```

### `azdo repo create [ORGANIZATION/]<PROJECT>/<NAME> [flags]`

Create a new repository in a project

```
-q, --jq expression          Filter JSON output using a jq expression
    --json fields[=*]        Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --parent string          [PROJECT/]REPO to fork from (same organization)
    --source-branch string   Only fork the specified branch (defaults to all branches)
-t, --template string        Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
cr
```

### `azdo repo delete [organization/]project/repository [flags]`

Delete a Git repository in a team project

```
-y, --yes   Do not prompt for confirmation
```

Aliases

```
d
```

### `azdo repo edit [organization/]project/repository [flags]`

Edit or update an existing Git repository in a team project

```
    --default-branch string   Set the default branch for the repository
    --disable                 Disable the repository
    --enable                  Enable the repository
-q, --jq expression           Filter JSON output using a jq expression
    --json fields[=*]         Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --name string             Rename the repository
-t, --template string         Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
e, update
```

### `azdo repo list [organization/]<project> [flags]`

List repositories of a project inside an organization

```
    --format string       Output format: {json} (default "table")
    --include-hidden      Include hidden repositories
-L, --limit int           Maximum number of repositories to list (default 30)
    --visibility string   Filter by repository visibility: {public|private}
```

Aliases

```
ls, l
```

### `azdo repo restore [organization/]project/repository`

Restore a deleted repository

Aliases

```
ls
```

### `azdo repo set-default [<repository>] [flags]`

Configure default repository for this directory

```
-u, --unset   unset the current default repository
-v, --view    view the current default repository
```

## `azdo security <command> [flags]`

Work with Azure DevOps security.

Aliases

```
s, sec
```

### `azdo security group`

Manage security groups

Aliases

```
g, grp
```

#### `azdo security group create [ORGANIZATION|ORGANIZATION/PROJECT] [flags]`

Create a security group

```
    --description string   Description of the new security group.
    --email string         Create a security group using an existing AAD group's email address.
    --groups strings       A comma-separated list of group descriptors to add the new group to.
-q, --jq expression        Filter JSON output using a jq expression
    --json fields[=*]      Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --name string          Name of the new security group.
    --origin-id string     Create a security group using an existing AAD group's origin ID.
-t, --template string      Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
add, new, c
```

#### `azdo security group delete [ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP] [flags]`

Delete an Azure DevOps security group

```
    --descriptor string   Descriptor of the group to delete (required if multiple groups match)
-y, --yes                 Do not prompt for confirmation
```

Aliases

```
d, del, rm
```

#### `azdo security group list [ORGANIZATION[/PROJECT]] [flags]`

List security groups

```
-f, --filter string           Case-insensitive regex to filter groups by name (e.g. 'dev.*team'). Invalid patterns will result in an error
-q, --jq expression           Filter JSON output using a jq expression
    --json fields[=*]         Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --subject-types strings   List of subject types to include (comma-separated). Values must not be empty or contain only whitespace.
-t, --template string         Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
ls, l
```

#### `azdo security group membership`

Manage security group memberships

Aliases

```
m
```

##### `azdo security group membership add [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP [flags]`

Add a member to an Azure DevOps security group.

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-m, --member strings    List of (comma-separated) Email, descriptor, or principal name of the user or group to add.
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
a, create, cr
```

##### `azdo security group membership list [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP [flags]`

List the members of an Azure DevOps security group.

```
-q, --jq expression         Filter JSON output using a jq expression
    --json fields[=*]       Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-r, --relationship string   Relationship type: members or memberof: {members|memberof} (default "members")
-t, --template string       Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
ls, l
```

##### `azdo security group membership remove [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP [flags]`

Remove a member from an Azure DevOps security group.

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-m, --member strings    List of (comma-separated) Email, descriptor, or principal name of the user or group to remove.
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
-y, --yes               Do not prompt for confirmation.
```

Aliases

```
d, r, rm, delete, del
```

#### `azdo security group show ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP [flags]`

Show details of an Azure DevOps security group

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
s
```

#### `azdo security group update ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP [flags]`

Update an Azure DevOps security group

```
    --description string   New description for the security group.
    --descriptor string    Descriptor of the security group (required if multiple groups match the name).
-q, --jq expression        Filter JSON output using a jq expression
    --json fields[=*]      Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --name string          New display name for the security group.
-t, --template string      Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
u
```

### `azdo security permission`

Manage Azure DevOps security permissions.

Aliases

```
p, perm, permissions
```

#### `azdo security permission delete <TARGET> [flags]`

Delete permissions for a user or group.

```
-n, --namespace-id string   ID of the security namespace to modify (required).
    --token string          Security token to delete (required).
-y, --yes                   Do not prompt for confirmation.
```

Aliases

```
d, del, rm
```

#### `azdo security permission list [TARGET] [flags]`

List security ACEs for a namespace, optionally filtered by subject.

```
-q, --jq expression         Filter JSON output using a jq expression
    --json fields[=*]       Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-n, --namespace-id string   ID of the security namespace to query (required).
    --recurse               Include child ACEs for the specified token when supported.
-t, --template string       Format JSON output using a Go template; see "azdo help formatting"
    --token string          Security token to filter the results.
```

Aliases

```
ls, l
```

#### `azdo security permission namespace`

Inspect security permission namespaces.

Aliases

```
n, ns
```

##### `azdo security permission namespace list [ORGANIZATION] [flags]`

List security permission namespaces.

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --local-only        Only include namespaces defined locally within the organization.
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
ls, l
```

##### `azdo security permission namespace show [ORGANIZATION/]NAMESPACE [flags]`

Show details for a security permission namespace.

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
s
```

#### `azdo security permission reset <TARGET> [flags]`

Reset explicit permission bits for a user or group.

```
-q, --jq expression            Filter JSON output using a jq expression
    --json fields[=*]          Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-n, --namespace-id string      ID of the security namespace to modify (required).
    --permission-bit strings   Permission bit or comma-separated bits to reset (required).
-t, --template string          Format JSON output using a Go template; see "azdo help formatting"
    --token string             Security token for the resource (required).
-y, --yes                      Do not prompt for confirmation.
```

Aliases

```
r
```

#### `azdo security permission show <TARGET> [flags]`

Show permissions for a user or group.

```
-q, --jq expression         Filter JSON output using a jq expression
    --json fields[=*]       Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-n, --namespace-id string   ID of the security namespace to query (required).
-t, --template string       Format JSON output using a Go template; see "azdo help formatting"
    --token string          Security token to query (required).
```

Aliases

```
s
```

#### `azdo security permission update <TARGET> [flags]`

Update or create permissions for a user or group.

```
    --allow-bit strings     Permission bit or comma-separated bits to allow.
    --deny-bit strings      Permission bit or comma-separated bits to deny.
    --merge                 Merge incoming ACEs with existing entries or replace the permissions. If provided without value true is implied.
-n, --namespace-id string   ID of the security namespace to modify (required).
    --token string          Security token for the resource (required).
-y, --yes                   Do not prompt for confirmation.
```

Aliases

```
create, u, new
```

## `azdo service-endpoint <command> [flags]`

Work with Azure DevOps service connections.

Aliases

```
service-endpoints, serviceendpoints, se
```

### `azdo service-endpoint create [ORGANIZATION/]PROJECT --from-file <path> [flags]`

Create service connections

```
-e, --encoding string    File encoding (utf-8, ascii, utf-16be, utf-16le). (default "utf-8")
-f, --from-file string   Path to the JSON service endpoint definition or '-' for stdin.
-q, --jq expression      Filter JSON output using a jq expression
    --json fields[=*]    Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-t, --template string    Format JSON output using a Go template; see "azdo help formatting"
```

Aliases

```
import
```

#### `azdo service-endpoint create azurerm [ORGANIZATION/]PROJECT --name <name> --authentication-scheme <scheme> [flags]`

Create an Azure Resource Manager service connection

```
    --authentication-scheme string        Authentication scheme: {ServicePrincipal|ManagedServiceIdentity|WorkloadIdentityFederation} (default "ServicePrincipal")
    --certificate-path string             Path to service principal certificate file (PEM format)
    --description string                  Description for the service endpoint
    --environment string                  Azure environment: {AzureCloud|AzureChinaCloud|AzureUSGovernment|AzureGermanCloud|AzureStack} (default "AzureCloud")
    --grant-permission-to-all-pipelines   Grant access permission to all pipelines to use the service connection
-q, --jq expression                       Filter JSON output using a jq expression
    --json fields[=*]                     Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --management-group-id string          Azure management group ID
    --management-group-name string        Azure management group name
    --name string                         Name of the service endpoint
    --resource-group string               Name of the resource group (for subscription-level scope)
    --server-url string                   Azure Stack Resource Manager base URL. Required if --environment is AzureStack.
    --service-principal-id string         Service principal/application ID (e.g., GUID)
    --service-principal-key string        Service principal key (secret value)
    --subscription-id string              Azure subscription ID (e.g., GUID)
    --subscription-name string            Azure subscription name
-t, --template string                     Format JSON output using a Go template; see "azdo help formatting"
    --tenant-id string                    Azure tenant ID (e.g., GUID)
-y, --yes                                 Skip confirmation prompts
```

Aliases

```
cr, c, new, n, add, a
```

#### `azdo service-endpoint create github [ORGANIZATION/]PROJECT --name NAME [--url URL] [--token TOKEN] [flags]`

Create a GitHub service endpoint

```
    --configuration-id string   Configuration for connecting to the endpoint (use an OAuth/installation configuration). Mutually exclusive with --token.
-q, --jq expression             Filter JSON output using a jq expression
    --json fields[=*]           Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --name string               Name of the service endpoint
-t, --template string           Format JSON output using a Go template; see "azdo help formatting"
    --token string              Visit https://github.com/settings/tokens to create personal access tokens. Recommended scopes: repo, user, admin:repo_hook. If omitted, you will be prompted for a token when interactive.
    --url string                GitHub URL (defaults to https://github.com)
```

### `azdo service-endpoint delete [ORGANIZATION/]PROJECT/ID_OR_NAME [flags]`

Delete a service endpoint from a project.

```
    --additional-project stringArray   Additional project scope [ORGANIZATION/]PROJECT when the endpoint is shared. (Repeatable, comma-separated)
    --deep                             Also delete the backing Azure AD application for supported endpoints.
-y, --yes                              Skip the confirmation prompt.
```

Aliases

```
rm, del, d
```

### `azdo service-endpoint export [ORGANIZATION/]PROJECT/ID_OR_NAME [flags]`

Export a service endpoint definition as JSON.

```
-o, --output-file string   Path to write the exported JSON. Defaults to stdout.
    --with-secrets         Include sensitive authorization values in the export.
```

Aliases

```
e, ex
```

### `azdo service-endpoint list [ORGANIZATION/]PROJECT [flags]`

List service endpoints in a project.

```
    --action-filter string   Filter endpoints by caller permissions (manage, use, view, none).
    --auth-scheme strings    Filter by authorization scheme. Repeat to specify multiple values or separate multiple values by comma ','.
    --endpoint-id strings    Filter by endpoint ID (UUID). Repeat to specify multiple values or separate multiple values by comma ','.
    --include-details        Request additional authorization metadata when available.
    --include-failed         Include endpoints that are in a failed state.
-q, --jq expression          Filter JSON output using a jq expression
    --json fields[=*]        Output JSON with the specified fields. Prefix a field with '-' to exclude it.
    --name strings           Filter by endpoint display name. Repeat to specify multiple values or separate multiple values by comma ','.
    --output-format string   Select non-JSON output format: {table|ids} (default "table")
    --owner string           Filter by service endpoint owner (e.g., Library, AgentCloud).
-t, --template string        Format JSON output using a Go template; see "azdo help formatting"
    --type string            Filter by service endpoint type (e.g., AzureRM, GitHub, Generic).
```

Aliases

```
ls, l
```

### `azdo service-endpoint show [ORGANIZATION/]PROJECT/ID_OR_NAME [flags]`

Show details of a service endpoint.

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields[=*]   Output JSON with the specified fields. Prefix a field with '-' to exclude it.
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
```



### See also

* [azdo](./azdo.md)
