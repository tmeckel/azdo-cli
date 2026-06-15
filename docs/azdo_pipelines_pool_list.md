## Command `azdo pipelines pool list`

```
azdo pipelines pool list [ORGANIZATION] [flags]
```

List Azure DevOps agent pools for an organization.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Optional client-side cap on results

* `--name` `string`

	Filter pools by name

* `--pool-type` `string`

	Filter pools by type: {automation|deployment}

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`agentCloudId`, `autoProvision`, `autoSize`, `autoUpdate`, `createdBy`, `createdOn`, `id`, `isHosted`, `isLegacy`, `name`, `options`, `owner`, `poolType`, `properties`, `scope`, `size`, `targetSize`

### Examples

```bash
# List all pools in the default organization
azdo pipelines pool list

# List pools in a specific organization
azdo pipelines pool list myorg

# List pools filtered by name
azdo pipelines pool list myorg --name Default

# List deployment pools
azdo pipelines pool list myorg --pool-type deployment

# Output as JSON
azdo pipelines pool list myorg --json
```

### See also

* [azdo pipelines pool](./azdo_pipelines_pool.md)
