## Command `azdo project show`

```
azdo project show [ORGANIZATION/]PROJECT [flags]
```

Shows details of an Azure DevOps project in the specified organization.

If the organization name is omitted from the project argument, the default configured organization is used.


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

`defaultTeamName`, `description`, `id`, `lastUpdateTime`, `name`, `process`, `revision`, `sourceControl`, `state`, `url`, `visibility`

### Examples

```bash
# Show project details in the default organization
azdo project show MyProject

# Show project details in a specific organization
azdo project show MyOrg/MyProject
```

### See also

* [azdo project](./azdo_project.md)
