## Command `azdo boards iteration project show`

```
azdo boards iteration project show [ORGANIZATION/]PROJECT[/PATH]/NAME [flags]
```

Display the details of a single iteration (sprint) node in a project.
The iteration is identified by its fully-qualified path under /Iteration.


### Options


* `--depth` `int` (default `0`)

	Depth of child nodes to fetch (0-10).

* `--include-children`

	Include child nodes in the template output.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-r`, `--raw`

	Dump the raw SDK node to stderr.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `view`
- `status`

### JSON Fields

`_links`, `attributes`, `children`, `hasChildren`, `id`, `identifier`, `name`, `path`, `structureType`, `url`

### Examples

```bash
# Show a top-level iteration
azdo boards iteration project show Fabrikam/Sprint\ 1

# Show a nested iteration
azdo boards iteration project show myorg/Fabrikam/Release\ 2025/Sprint\ 1

# Include child nodes in the template output
azdo boards iteration project show Fabrikam/Release\ 2025 --include-children

# Emit the raw SDK node as JSON
azdo boards iteration project show Fabrikam/Sprint\ 1 --json
```

### See also

* [azdo boards iteration project](./azdo_boards_iteration_project.md)
