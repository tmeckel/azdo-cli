## Command `azdo boards work-item relation show`

```
azdo boards work-item relation show [ORG:]PROJECT/ID [flags]
```

List all relations of an existing work item. Relation types are
displayed by their friendly name.


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

`_links`, `commentVersionRef`, `fields`, `id`, `relations`, `rev`, `url`

### Examples

```bash
# List the relations of a work item
azdo boards work-item relation show Fabrikam/1234
```

### See also

* [azdo boards work-item relation](./azdo_boards_work-item_relation.md)
