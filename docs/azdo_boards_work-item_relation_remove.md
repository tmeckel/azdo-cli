## Command `azdo boards work-item relation remove`

```
azdo boards work-item relation remove [ORG:]PROJECT/ID [flags]
```

Detach one or more relations from an existing work item. The relation
type must be one of the friendly names returned by 'list-type'.
Targets are specified by work item ID.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--relation-type` `string`

	Relation type (friendly name, e.g. parent, child, related).

* `--target-id` `stringArray`

	Target work item ID (repeatable; comma-separated values accepted).

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-y`, `--yes`

	Skip the confirmation prompt.


### ALIASES

- `r`
- `rm`

### JSON Fields

`_links`, `commentVersionRef`, `fields`, `id`, `relations`, `rev`, `url`

### Examples

```bash
# Remove a parent relation to another work item
azdo boards work-item relation remove Fabrikam/1234 --relation-type parent --target-id 5678 --yes

# Remove relations to multiple work items
azdo boards work-item relation remove Fabrikam/1234 --relation-type related --target-id 5678,5679 --yes
```

### See also

* [azdo boards work-item relation](./azdo_boards_work-item_relation.md)
