## Command `azdo pipelines folder list`

```
azdo pipelines folder list [ORGANIZATION/]PROJECT [flags]
```

List build definition folders in PROJECT.

Mirrors 'az pipelines folder list'. Use --path to limit the listing to
a sub-folder. Use --query-order to sort by path ascending or descending.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Maximum number of folders to return (client-side; 0 = unlimited)

* `--path` `string`

	Limit the listing to folders at or under this path.

* `--query-order` `string`

	Sort folders by path: {asc|desc}

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`description`, `path`

### Examples

```bash
# List top-level folders in a project
azdo pipelines folder list Fabrikam

# List folders at or under a sub-path
azdo pipelines folder list Fabrikam --path /Shared

# List folders sorted descending by path
azdo pipelines folder list myorg/Fabrikam --query-order desc

# Output as JSON
azdo pipelines folder list Fabrikam --json
```

### See also

* [azdo pipelines folder](./azdo_pipelines_folder.md)
