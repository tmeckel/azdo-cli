## Command `azdo pipelines variable-group delete`

```
azdo pipelines variable-group delete [ORGANIZATION/]PROJECT/GROUP [flags]
```

Delete a variable group from a project using its numeric ID or name. The command prompts
for confirmation unless --yes is supplied.


### Options


* `--all`

	Remove the variable group from every assigned project

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--project-reference` `strings`

	Additional project names or IDs to remove the group from (repeatable, comma-separated)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-y`, `--yes`

	Skip the confirmation prompt.


### ALIASES

- `rm`
- `del`
- `d`

### JSON Fields

`deleted`, `groupId`

### Examples

```bash
# Delete a variable group by ID in the default organization
azdo pipelines variable-group delete MyProject/123 --yes

# Delete a variable group by name in a specific organization
azdo pipelines variable-group delete 'myorg/MyProject/Shared Config'

# Remove a shared group from two additional projects
azdo pipelines variable-group delete MyProject/SharedConfig --project-reference ProjectB --project-reference ProjectC

# Remove a group from every project assignment
azdo pipelines variable-group delete MyProject/SharedConfig --all --yes
```

### See also

* [azdo pipelines variable-group](./azdo_pipelines_variable-group.md)
