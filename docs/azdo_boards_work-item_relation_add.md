## Command `azdo boards work-item relation add`

```
azdo boards work-item relation add [ORG:]PROJECT/ID [flags]
```

Attach one or more relations to an existing work item. The relation type
must be one of the friendly names returned by 'list-type'. Targets can
be other work items (by ID, optionally prefixed with their project) or
arbitrary artifact URLs. Work items in other projects of the same
organization are resolved via 'PROJECT/ID'. Cross-organization links
are not possible by ID; use --target-url with a remote link type such
as 'Remote Related', 'Consumes From' or 'Produces For'.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--relation-type` `string`

	Relation type (friendly name, e.g. parent, child, related).

* `-T`, `--target-id` `stringArray`

	Target work item ID (repeatable; comma-separated; each entry is [PROJECT/]ID; ID-only targets resolve in the current project).

* `-u`, `--target-url` `stringArray`

	Target artifact URL (repeatable; comma-separated values accepted).

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `a`

### JSON Fields

`_links`, `commentVersionRef`, `fields`, `id`, `relations`, `rev`, `url`

### Examples

```bash
# Add a parent relation to another work item
azdo boards work-item relation add Fabrikam/1234 --relation-type parent --target-id 5678

# Add a parent relation to a work item in another project of the same organization
azdo boards work-item relation add Fabrikam/1234 --relation-type parent --target-id Contoso/77

# Add a relation to multiple work items
azdo boards work-item relation add Fabrikam/1234 --relation-type related --target-id 5678,5679

# Add an artifact relation
azdo boards work-item relation add Fabrikam/1234 --relation-type artifact --target-url https://example.com/release
```

### See also

* [azdo boards work-item relation](./azdo_boards_work-item_relation.md)
