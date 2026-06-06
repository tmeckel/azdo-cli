## Command `azdo team show`

```
azdo team show [ORGANIZATION/]PROJECT/TEAM [flags]
```

Show details of a single team in a project. The team is identified by its
name or GUID inside the project. The organization falls back to the
configured default when omitted.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `s`

### JSON Fields

`description`, `id`, `identity`, `identityUrl`, `name`, `projectId`, `projectName`, `url`

### Examples

```bash
# Show a team by name in the default organization
azdo team show Fabrikam/"Fabrikam Engineering"

# Show a team by ID in a specific organization
azdo team show MyOrg/Fabrikam/00000002-0000-0000-0000-000000000000
```

### See also

* [azdo team](./azdo_team.md)
