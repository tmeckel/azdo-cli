## Command `azdo team update`

```
azdo team update [ORGANIZATION/]PROJECT/TEAM [flags]
```

Update a team's name and/or description. At least one of --name or
--description must be provided. The team is identified by its name or
GUID inside the project.


### Options


* `--description` `string`

	New description of the team

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `string`

	New name of the team

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `u`

### JSON Fields

`description`, `id`, `identity`, `identityUrl`, `name`, `projectId`, `projectName`, `url`

### Examples

```bash
# Rename a team
azdo team update Fabrikam/"Old Name" --name "New Name"

# Update a team's description only
azdo team update MyOrg/Fabrikam/MyTeam --description "New description"
```

### See also

* [azdo team](./azdo_team.md)
