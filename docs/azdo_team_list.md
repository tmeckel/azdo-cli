## Command `azdo team list`

```
azdo team list [ORGANIZATION/]PROJECT [flags]
```

List all teams in the specified project. Supports server-side paging via
--top and --skip, --mine filtering, and JSON export.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Maximum number of teams to return across all pages (client-side; 0 = unlimited)

* `--mine`

	Return only teams the current user is a member of

* `--skip` `int` (default `0`)

	Number of teams to skip (server-side)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--top` `int` (default `0`)

	Maximum number of teams to return per page (server-side; 0 = server default)


### ALIASES

- `ls`
- `l`

### JSON Fields

`description`, `id`, `identity`, `identityUrl`, `name`, `projectId`, `projectName`, `url`

### Examples

```bash
# List all teams in the default organization
azdo team list Fabrikam

# List the first 10 teams in a specific organization
azdo team list MyOrg/Fabrikam --top 10

# List teams you are a member of
azdo team list Fabrikam --mine
```

### See also

* [azdo team](./azdo_team.md)
