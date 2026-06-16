## Command `azdo pipelines queue show`

```
azdo pipelines queue show [ORGANIZATION/]PROJECT/QUEUE [flags]
```

Display the details of a single Azure DevOps agent queue.
The queue is identified by integer ID or name, with an
optional organization prefix.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `view`
- `status`

### JSON Fields

`id`, `name`, `pool`, `projectId`

### Examples

```bash
# Show a queue by ID
azdo pipelines queue show Fabrikam/7

# Show a queue by name
azdo pipelines queue show 'Fabrikam/Default'

# Show a queue in a specific organization
azdo pipelines queue show 'myorg/Fabrikam/Default'
```

### See also

* [azdo pipelines queue](./azdo_pipelines_queue.md)
