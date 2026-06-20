## Command `azdo boards iteration project delete`

```
azdo boards iteration project delete [ORGANIZATION/]PROJECT[/PATH]/NAME [flags]
```

Delete an iteration (sprint) from a project. The command prompts for
confirmation unless --yes is supplied. Use --reclassify-id to move any
work items to another node before deletion; the Azure DevOps REST API
rejects deletes while a node is still in use unless work items are
reclassified first.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-r`, `--reclassify-id` `int`

	ID of the target node to which work items should be moved before deletion.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-y`, `--yes`

	Skip the confirmation prompt.


### ALIASES

- `d`
- `del`
- `rm`

### JSON Fields

`deleted`, `path`, `reclassifyId`

### Examples

```bash
# Delete a top-level iteration
azdo boards iteration project delete Fabrikam/Sprint\ 1 --yes

# Delete a nested iteration with a confirmation prompt
azdo boards iteration project delete Fabrikam/Release\ 2025/Sprint\ 1

# Reclassify work items to node 42 before deletion
azdo boards iteration project delete Fabrikam/Sprint\ 1 \
	--reclassify-id 42 --yes

# Emit JSON
azdo boards iteration project delete Fabrikam/Sprint\ 1 --reclassify-id 42 --json
```

### See also

* [azdo boards iteration project](./azdo_boards_iteration_project.md)
