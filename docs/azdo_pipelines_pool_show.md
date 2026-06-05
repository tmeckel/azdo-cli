## Command `azdo pipelines pool show`

```
azdo pipelines pool show [ORGANIZATION/]POOL [flags]
```

Display the details of a single Azure DevOps agent pool.
The pool is identified by integer ID or name, with an
optional organization prefix.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-r`, `--raw`

	Dump raw pool object to stderr

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `view`
- `status`

### JSON Fields

`autoProvision`, `autoUpdate`, `createdBy`, `createdOn`, `id`, `isHosted`, `isLegacy`, `name`, `options`, `owner`, `poolType`, `properties`, `scope`, `size`, `targetSize`

### Examples

```bash
# Show a pool by ID
azdo pipelines pool show 42

# Show a pool by name
azdo pipelines pool show 'Default'

# Show a pool in a specific organization
azdo pipelines pool show 'myorg/Default'
```

### See also

* [azdo pipelines pool](./azdo_pipelines_pool.md)
