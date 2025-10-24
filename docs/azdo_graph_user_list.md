## Command `azdo graph user list`

```
azdo graph user list [project] [flags]
```

List users and groups from an Azure DevOps project or organization.

By default, it lists users in the organization. You can scope the search to a specific
project by providing the project name as an argument.

The command allows filtering by user type (e.g., 'aad', 'msa', 'svc') and supports
prefix-based filtering on user display names.


### Options


* `-F`, `--filter` `string`

	Filter users by prefix (max 100 results)

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-L`, `--limit` `int` (default `20`)

	Maximum number of users to return (pagination client-side)

* `-o`, `--organization` `string`

	Organization name. If not specified, defaults to the default organization

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-T`, `--type` `strings`

	Subject types filter (comma-separated). If not specified defaults to &#39;aad&#39;


### ALIASES

- `ls`

### JSON Fields

`descriptor`, `displayName`, `mailAddress`, `origin`, `principalName`

### Examples

```bash
# List all users in the default organization
azdo graph user list

# List all users in a specific project
azdo graph user list "My Project"

# List all users with the 'msa' subject type (Microsoft Account)
azdo graph user list --type msa

# Filter users by a name prefix
azdo graph user list --filter "john.doe"

# Limit the number of users returned
azdo graph user list --limit 10

# List users in a specific organization
azdo graph user list --organization "MyOrganization"

# Output the result as JSON
azdo graph user list --json
```

### See also

* [azdo graph user](./azdo_graph_user.md)
