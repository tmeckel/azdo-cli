## Command `azdo pipelines variable-group list`

```
azdo pipelines variable-group list [ORGANIZATION/]PROJECT [flags]
```

List every variable group defined in a project with optional filtering.


### Options


* `--action` `string`

	Action filter string (e.g., &#39;manage&#39;, &#39;use&#39;): {none|manage|use}

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Optional client-side cap on results; stop fetching once reached

* `--order` `string` (default `&#34;desc&#34;`)

	Order of variable groups (asc, desc): {desc|asc}

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--top` `int` (default `0`)

	Server-side page size hint (positive integer)


### ALIASES

- `ls`
- `l`

### JSON Fields

`createdBy`, `createdOn`, `description`, `id`, `isShared`, `modifiedBy`, `modifiedOn`, `name`, `projectReferences`, `type`, `variables`

### Examples

```bash
# List all variable groups in a project
$ azdo pipelines variable-groups list "my-project"

# List variable groups with a specific name
$ azdo pipelines variable-groups list "my-project" --name "my-variable-group"
```

### See also

* [azdo pipelines variable-group](./azdo_pipelines_variable-group.md)
