## azdo reference
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
````

### `azdo auth logout [ORG]`

Log out of a Azure DevOps organization

### `azdo auth setup-git [flags]`

Setup git with AzDO CLI

```
-o, --organization string   Configure git credential helper for specific organization
````

### `azdo auth status [organization]`

View authentication status

## `azdo co`

Alias for "pr checkout"

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
    --all                   Show config options which are not configured
-o, --organization string   Get per-organization configuration
````

### `azdo config set <key> <value> [flags]`

Update configuration with a value for the given key

```
-o, --organization string   Set per-organization setting
-r, --remove                Remove config item for an organization, so that the default value will be in effect again
````

## `azdo graph <command>`

Manage Azure DevOps Graph resources (users, groups)

### `azdo graph user <command>`

Manage users in Azure DevOps

#### `azdo graph user list [project] [flags]`

List users and groups in Azure DevOps

```
-F, --filter string         Filter users by prefix (max 100 results)
-q, --jq expression         Filter JSON output using a jq expression
    --json fields           Output JSON with the specified fields
-L, --limit int             Maximum number of users to return (pagination client-side) (default 20)
-o, --organization string   Organization name. If not specified, defaults to the default organization
-t, --template string       Format JSON output using a Go template; see "azdo help formatting"
-T, --type strings          Subject types filter (comma-separated). If not specified defaults to 'aad'
````

## `azdo pr <command> [flags]`

Manage pull requests

```
--help   Show help for command
````

### `azdo pr checkout <number> [flags]`

Check out a pull request in git

```
-b, --branch string        Local branch name to use (default [the name of the head branch])
    --detach               Checkout PR with a detached HEAD
-f, --force                Reset the existing local branch to the latest state of the pull request
    --recurse-submodules   Update all submodules after checkout
````

### `azdo pr close <number> | <branch> | <url> [flags]`

Close a pull request

```
-c, --comment string   Leave a closing comment
-d, --delete-branch    Delete the local and remote branch after close
````

### `azdo pr comment [<number> | <branch> | <url>] [flags]`

Comment a pull request

```
-c, --comment string   Comment to add to the pull request. Use '-' to read from stdin.
-t, --thread int       ID of the thread to reply to.
````

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
````

### `azdo pr diff [<number> | <branch> | <url>] [flags]`

View changes in a pull request

```
--color string   Use color in diff output: {always|never|auto} (default "auto")
--name-only      Display only names of changed files
````

### `azdo pr edit [<number> | <branch> | <url>] [flags]`

Edit a pull request

```
    --add-label strings                  Add labels (comma-separated)
    --add-optional-reviewer strings      Add optional reviewers (comma-separated)
    --add-required-reviewer strings      Add required reviewers (comma-separated)
-B, --base string                        Change the base branch for this pull request
-b, --body string                        Set the new body.
-F, --body-file string                   Read body text from file (use "-" to read from standard input)
    --remove-label strings               Remove labels (comma-separated)
    --remove-optional-reviewer strings   Remove optional reviewers (comma-separated)
    --remove-required-reviewer strings   Remove required reviewers (comma-separated)
-t, --title string                       Set the new title.
````

### `azdo pr list [[organization/]project/repository] [flags]`

List pull requests in a repository or a project

```
-a, --author string       Filter by author
-B, --base string         Filter by base branch
-d, --draft               Filter by draft state
-H, --head string         Filter by head branch
-q, --jq expression       Filter JSON output using a jq expression
    --json fields         Output JSON with the specified fields
-l, --label strings       Filter by label
-L, --limit int           Maximum number of items to fetch (default 30)
-m, --mergestate string   Filter by merge state: {succeeded|conflicts}
-r, --reviewer string     Filter by reviewer
-s, --state string        Filter by state: {abandoned|active|all|completed} (default "active")
-t, --template string     Format JSON output using a Go template; see "azdo help formatting"
````

### `azdo pr merge <number> | <branch> | <url> [flags]`

Merge a pull request

```
-d, --delete-source-branch    Delete the source branch after merging
    --merge-strategy string   Merge strategy to use: {noFastForward|squash|rebase|rebaseMerge} (default "NoFastForward")
-m, --message string          Message to include when completing the pull request
    --transition-work-items   Transition linked work item statuses upon merging (default true)
````

### `azdo pr reopen <number> | <branch> | <url> [flags]`

Reopen a pull request

```
-c, --comment string   Add a reopening comment
````

### `azdo pr status [flags]`

Show status of relevant pull requests

```
-c, --conflict-status   Display the merge conflict status of each pull request
-q, --jq expression     Filter JSON output using a jq expression
    --json fields       Output JSON with the specified fields
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
````

### `azdo pr view [<number> | <branch> | <url>] [flags]`

View a pull request

```
    --comment-sort string   Sort comments by creation time; defaults to 'desc' (newest first): {desc|asc} (default "desc")
    --comment-type string   Filter comments by type; defaults to 'text': {text|system|all} (default "text")
-c, --comments              View pull request comments
-C, --commits               View pull request commits
    --format string         Output format: {json}
-q, --jq expression         Filter JSON output using a jq expression
-r, --raw                   View pull request raw
-t, --template string       Format JSON output using a Go template; see "azdo help formatting"
````

### `azdo pr vote [<number> | <branch> | <url>] [flags]`

Vote on a pull request

```
--vote string   Vote value to set: {approve|approve-with-suggestions|reject|reset|wait-for-author} (default "approve")
````

## `azdo project <command> [flags]`

Work with Azure DevOps Projects.

### `azdo project create [ORGANIZATION/]PROJECT [flags]`

Create a new Azure DevOps Project

```
-d, --description string      Description for the new project
-q, --jq expression           Filter JSON output using a jq expression
    --json fields             Output JSON with the specified fields
    --max-wait int            Maximum wait time in seconds (default 3600)
    --no-wait                 Do not wait for the project to be created
-p, --process string          Process to use (e.g., Scrum, Agile, CMMI) (default "Agile")
-s, --source-control string   Source control type (git or tfvc) (default "git")
-t, --template string         Format JSON output using a Go template; see "azdo help formatting"
    --visibility string       Project visibility (private or public) (default "private")
````

### `azdo project delete [ORGANIZATION/]PROJECT [flags]`

Delete a project

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields       Output JSON with the specified fields
    --max-wait int      Maximum wait time in seconds (default 3600)
    --no-wait           Do not wait for the project deletion to complete
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
-y, --yes               Skip confirmation prompt
````

### `azdo project list [organization] [flags]`

List the projects for an organization

```
    --format string   Output format: {json} (default "table")
-l, --limit int       Maximum number of projects to fetch (default 30)
    --state string    Project state filter: {deleting|new|wellFormed|createPending|all|unchanged|deleted}
````

### `azdo project show [ORGANIZATION/]PROJECT [flags]`

Show details of an Azure DevOps Project

```
-q, --jq expression     Filter JSON output using a jq expression
    --json fields       Output JSON with the specified fields
-t, --template string   Format JSON output using a Go template; see "azdo help formatting"
````

## `azdo repo <command>`

Manage repositories

### `azdo repo clone [organization/]project/repository [<directory>] [-- <gitflags>...]`

Clone a repository locally

```
    --no-credential-helper          Don't configure azdo as credential helper for the cloned repository
    --recurse-submodules            Update all submodules after checkout
-u, --upstream-remote-name string   Upstream remote name when cloning a fork (default "upstream")
````

### `azdo repo create [ORGANIZATION/]<PROJECT>/<NAME> [flags]`

Create a new repository in a project

```
-q, --jq expression          Filter JSON output using a jq expression
    --json fields            Output JSON with the specified fields
    --parent string          [PROJECT/]REPO to fork from (same organization)
    --source-branch string   Only fork the specified branch (defaults to all branches)
-t, --template string        Format JSON output using a Go template; see "azdo help formatting"
````

### `azdo repo delete [organization/]project/repository [flags]`

Delete a Git repository in a team project

```
-y, --yes   Do not prompt for confirmation
````

### `azdo repo edit [organization/]project/repository [flags]`

Edit or update an existing Git repository in a team project

```
    --default-branch string   Set the default branch for the repository
    --disable                 Disable the repository
    --enable                  Enable the repository
-q, --jq expression           Filter JSON output using a jq expression
    --json fields             Output JSON with the specified fields
    --name string             Rename the repository
-t, --template string         Format JSON output using a Go template; see "azdo help formatting"
````

### `azdo repo list [organization/]<project> [flags]`

List repositories of a project inside an organization

```
    --format string       Output format: {json} (default "table")
    --include-hidden      Include hidden repositories
-L, --limit int           Maximum number of repositories to list (default 30)
    --visibility string   Filter by repository visibility: {public|private}
````

### `azdo repo restore [organization/]project/repository`

Restore a deleted repository

### `azdo repo set-default [<repository>] [flags]`

Configure default repository for this directory

```
-u, --unset   unset the current default repository
-v, --view    view the current default repository
````

## `azdo security <command> [flags]`

Work with Azure DevOps security.

### `azdo security group`

Manage security groups

#### `azdo security group create [ORGANIZATION|ORGANIZATION/PROJECT] [flags]`

Create a security group

```
    --description string   Description of the new security group.
    --email string         Create a security group using an existing AAD group's email address.
    --groups strings       A comma-separated list of group descriptors to add the new group to.
-q, --jq expression        Filter JSON output using a jq expression
    --json fields          Output JSON with the specified fields
    --name string          Name of the new security group.
    --origin-id string     Create a security group using an existing AAD group's origin ID.
-t, --template string      Format JSON output using a Go template; see "azdo help formatting"
````

#### `azdo security group delete [ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP] [flags]`

Delete an Azure DevOps security group

```
    --descriptor string   Descriptor of the group to delete (required if multiple groups match)
-y, --yes                 Do not prompt for confirmation
````

#### `azdo security group list [ORGANIZATION[/PROJECT]] [flags]`

List security groups

```
-f, --filter string           Case-insensitive regex to filter groups by name (e.g. 'dev.*team'). Invalid patterns will result in an error
-q, --jq expression           Filter JSON output using a jq expression
    --json fields             Output JSON with the specified fields
    --subject-types strings   List of subject types to include (comma-separated). Values must not be empty or contain only whitespace.
-t, --template string         Format JSON output using a Go template; see "azdo help formatting"
````


### See also

* [azdo](./azdo.md)
