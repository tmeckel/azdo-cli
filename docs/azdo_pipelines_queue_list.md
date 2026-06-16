## Command `azdo pipelines queue list`

```
azdo pipelines queue list [ORGANIZATION/]PROJECT [flags]
```

List agent queues in an Azure DevOps project.


### Options


* `--action-filter` `string`

	Filter queues by caller permissions: {manage|none|use}

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Optional client-side cap on results

* `--name` `string`

	Filter queues by name

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`id`, `name`, `pool`, `projectId`

### Examples

```bash
# List all queues in a project
azdo pipelines queue list Fabrikam

# List queues in a specific organization
azdo pipelines queue list myorg/Fabrikam

# List queues filtered by name
azdo pipelines queue list myorg/Fabrikam --name Default

# Output as JSON
azdo pipelines queue list Fabrikam --json
```

### See also

* [azdo pipelines queue](./azdo_pipelines_queue.md)
