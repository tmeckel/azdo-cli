## Command `azdo boards iteration project create`

Create an iteration (sprint) in a project.

```
azdo boards iteration project create [ORGANIZATION/]PROJECT[/PATH]/NAME [flags]
```

### Options


* `--attributes` `strings`

	Custom attribute in key=value form. Repeatable. start-date/finish-date win on key conflict.

* `--finish-date` `string`

	Iteration finish date (RFC 3339 or YYYY-MM-DD).

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--start-date` `string`

	Iteration start date (RFC 3339 or YYYY-MM-DD).

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `c`
- `cr`

### JSON Fields

`_links`, `attributes`, `hasChildren`, `id`, `identifier`, `name`, `path`, `structureType`, `url`

### Examples

```bash
# Create a top-level iteration
azdo boards iteration project create Fabrikam/Sprint\ 1

# Schedule a sprint with start and finish dates
azdo boards iteration project create Fabrikam/Sprint\ 2 \
	--start-date 2025-01-06 --finish-date 2025-01-19

# Create a nested iteration under an existing release
azdo boards iteration project create myorg/Fabrikam/Release\ 2025/Sprint\ 2

# Set a custom attribute alongside the dates
azdo boards iteration project create Fabrikam/Sprint\ 1 \
	--start-date 2025-01-06 --finish-date 2025-01-19 \
	--attributes goal="Ship login"

# Emit JSON
azdo boards iteration project create Fabrikam/Sprint\ 1 --json
```

### See also

* [azdo boards iteration project](./azdo_boards_iteration_project.md)
