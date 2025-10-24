## Command `azdo project delete`

Delete a project

```
azdo project delete [ORGANIZATION/]PROJECT [flags]
```

### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-wait` `int` (default `3600`)

	Maximum wait time in seconds

* `--no-wait`

	Do not wait for the project deletion to complete

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-y`, `--yes`

	Skip confirmation prompt


### ALIASES

- `d`

### JSON Fields

`ID`, `Status`, `Url`

### Examples

```bash
# delete a project in the default organization
azdo project delete myproject

# delete a project in a specific organization
azdo project delete myorg/myproject```

### See also

* [azdo project](./azdo_project.md)
