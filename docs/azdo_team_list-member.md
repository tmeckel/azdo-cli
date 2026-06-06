## Command `azdo team list-member`

```
azdo team list-member [ORGANIZATION/]PROJECT/TEAM [flags]
```

List members of a team. The TEAM argument accepts the ID (GUID)
or name of the team. Supports server-side paging via --top and
--skip.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Maximum number of members to return across all pages (client-side; 0 = unlimited)

* `--skip` `int` (default `0`)

	Number of members to skip (server-side)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--top` `int` (default `0`)

	Maximum number of members to return per page (server-side; 0 = server default)


### ALIASES

- `members`

### JSON Fields

`identity`, `isTeamAdmin`

### Examples

```bash
# List members of a team
azdo team list-member Fabrikam/"Fabrikam Engineering"

# List the first 10 members in a specific organization
azdo team list-member MyOrg/Fabrikam/MyTeam --top 10
```

### See also

* [azdo team](./azdo_team.md)
