## Command `azdo boards work-item show`

```
azdo boards work-item show [ORG:]PROJECT/ID [flags]
```

Display the details of a single Azure Boards work item by its integer
ID. The work item is fetched with Expand=All so relations, fields and
links are returned in one call. The description is rendered
format-aware: Markdown content is passed through, Html content is
converted to Markdown first.


### Options


* `--comments`

	Fetch and render the work item&#39;s comment thread

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--relations`

	Render the work item&#39;s relations block

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `view`
- `status`

### JSON Fields

`_links`, `commentVersionRef`, `fields`, `id`, `relations`, `rev`, `url`

### Examples

```bash
# Show work item 12345 in the default organization's Fabrikam project
azdo boards work-item show Fabrikam/12345

# Show a work item in a specific organization
azdo boards work-item show myorg:Fabrikam/12345

# Include the work item's comment thread and relations
azdo boards work-item show Fabrikam/12345 --comments --relations

# Export the work item as JSON
azdo boards work-item show Fabrikam/12345 --json
```

### See also

* [azdo boards work-item](./azdo_boards_work-item.md)
