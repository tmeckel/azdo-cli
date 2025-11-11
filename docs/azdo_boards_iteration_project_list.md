## Command `azdo boards iteration project list`

```
azdo boards iteration project list [ORGANIZATION/]PROJECT [flags]
```

List the iteration (sprint) hierarchy for a project within an Azure DevOps organization.


### Options


* `-d`, `--depth` `int` (default `3`)

	Depth to fetch (1-10)

* `--finish-date` `string`

	Apply a comparison filter to iteration finish dates; supports operators like &lt;= and special value &#34;today&#34; (e.g., &#34;&lt;=today&#34;)

* `--include-dates`

	Include iteration start and finish dates

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-p`, `--path` `string`

	Iteration path relative to project root

* `--start-date` `string`

	Apply a comparison filter to iteration start dates; supports operators like &gt;= and special value &#34;today&#34; (e.g., &#34;&gt;=today&#34;)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`finishDate`, `hasChildren`, `level`, `name`, `path`, `startDate`

### Examples

```bash
# List the top-level iterations (depth 3)
azdo boards iteration project list myorg/myproject

# List from a specific path
azdo boards iteration project list myproject --path "Release 2025/Sprint 1"

# Include start and finish dates
azdo boards iteration project list myproject --include-dates

# List iterations starting today or later
azdo boards iteration project list myproject --start-date ">=today"

# Filter to iterations finishing before a specific date
azdo boards iteration project list myproject --finish-date "<=2024-12-31"

# Export JSON
azdo boards iteration project list myproject --json name,path,startDate
```

### See also

* [azdo boards iteration project](./azdo_boards_iteration_project.md)
