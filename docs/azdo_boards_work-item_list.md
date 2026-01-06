## Command `azdo boards work-item list`

```
azdo boards work-item list [ORGANIZATION/]PROJECT [flags]
```

List work items belonging to a project within an Azure DevOps organization.

This command builds and runs a WIQL query to obtain work item IDs and then fetches the
work item details in batches.


### Options


* `--area` `strings`

	Filter by area path (repeatable); prefix with Under: to include subtree (e.g., Under:Web/Payments)

* `-a`, `--assigned-to` `strings`

	Filter by assigned-to identity (repeatable); supports emails, descriptors, and @me

* `-c`, `--classification` `strings`

	Filter by severity classification (repeatable): 1 - Critical, 2 - High, 3 - Medium, 4 - Low

* `--iteration` `strings`

	Filter by iteration path (repeatable); prefix with Under: to include subtree (e.g., Under:Release 2025/Sprint 1)

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-L`, `--limit` `int` (default `50`)

	Maximum number of results to return (&gt;=1)

* `-p`, `--priority` `ints`

	Filter by priority (repeatable): 1-4

* `-s`, `--status` `strings` (default `[open]`)

	Filter by state category: open, closed, resolved, all (repeatable)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-T`, `--type` `strings`

	Filter by work item type (repeatable)


### ALIASES

- `ls`
- `l`

### JSON Fields

`_links`, `commentVersionRef`, `fields`, `id`, `relations`, `rev`, `url`

### Examples

```bash
# List open work items for a project in the default organization
azdo boards work-item list Fabrikam

# List all work items assigned to you
azdo boards work-item list Fabrikam --assigned-to @me --status all

# Filter by work item type and priority
azdo boards work-item list Fabrikam --type "User Story" --priority 1 --priority 2

# Filter by area subtree
azdo boards work-item list Fabrikam --area Under:Web/Payments

# Export JSON
azdo boards work-item list Fabrikam --json id,fields
```

### See also

* [azdo boards work-item](./azdo_boards_work-item.md)
