## Command `azdo team delete`

```
azdo team delete [ORGANIZATION/]PROJECT/TEAM [flags]
```

Delete a team from a project.

The TEAM argument accepts the ID (GUID) or name of the team.
A confirmation prompt is shown unless --yes is provided.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-y`, `--yes`

	Skip confirmation prompt


### ALIASES

- `d`
- `del`
- `rm`

### Examples

```bash
# Delete a team (with confirmation)
azdo team delete Fabrikam/"Old Team"

# Delete a team without confirmation
azdo team delete MyOrg/Fabrikam/00000002-0000-0000-0000-000000000000 --yes
```

### See also

* [azdo team](./azdo_team.md)
