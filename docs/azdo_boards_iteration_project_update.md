## Command `azdo boards iteration project update`

```
azdo boards iteration project update [ORGANIZATION/]PROJECT[/PATH]/NAME [flags]
```

Update an iteration (sprint) in a project. The positional argument identifies
the iteration as [ORGANIZATION/]PROJECT[/PATH]/NAME.

Supports changing start/finish dates and setting arbitrary attributes.


### Options


* `--attributes` `strings`

	Custom attribute in key=value form. Repeatable. Existing attributes not mentioned are preserved.

* `--finish-date` `string`

	New finish date (RFC 3339 or YYYY-MM-DD). Wins on conflict with --attributes finishDate. Must be on or after start-date when both are set.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--start-date` `string`

	New start date (RFC 3339 or YYYY-MM-DD). Wins on conflict with --attributes startDate.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `u`
- `up`

### JSON Fields

`_links`, `attributes`, `hasChildren`, `id`, `identifier`, `name`, `path`, `structureType`, `url`

### Examples

```bash
# Reschedule a sprint
azdo boards iteration project update Fabrikam/Sprint\ 1 \
	--start-date 2025-01-06 --finish-date 2025-01-19

# Add or change a custom attribute, keeping the existing dates
azdo boards iteration project update Fabrikam/Release\ 2025/Sprint\ 1 \
	--attributes goal="Ship login"

# Combine: reschedule + set a custom attribute
azdo boards iteration project update myorg/Fabrikam/Release\ 2025/Sprint\ 1 \
	--start-date 2025-01-06 --finish-date 2025-01-19 \
	--attributes goal="Ship login"

# Emit JSON
azdo boards iteration project update Fabrikam/Sprint\ 1 --json
```

### See also

* [azdo boards iteration project](./azdo_boards_iteration_project.md)
