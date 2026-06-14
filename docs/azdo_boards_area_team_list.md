## Command `azdo boards area team list`

```
azdo boards area team list [ORGANIZATION/]PROJECT/TEAM [flags]
```

List Azure Boards area paths assigned to a team. The TEAM argument accepts
the ID (GUID) or name of the team. The argument accepts the form
[ORGANIZATION/]PROJECT/TEAM. When the organization segment is omitted,
the default organization from configuration is used.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`areaPath`, `includeChildren`, `isDefault`

### Examples

```bash
# List area paths for a team using the default organization
azdo boards area team list Fabrikam/"Fabrikam Engineering"

# List area paths for a team in a specific organization
azdo boards area team list MyOrg/Fabrikam/"My Team"
```

### See also

* [azdo boards area team](./azdo_boards_area_team.md)
