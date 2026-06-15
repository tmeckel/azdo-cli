## Command `azdo pipelines show`

```
azdo pipelines show [ORGANIZATION/]PROJECT/PIPELINE [flags]
```

Display the details of a single Azure Pipelines definition.

The pipeline may be specified by ID (integer) or name (string).
When the organization segment is omitted the default organization
from configuration is used.


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

`_links`, `authoredBy`, `createdDate`, `description`, `id`, `name`, `path`, `process`, `quality`, `queue`, `repository`, `revision`, `type`, `url`

### Examples

```bash
# Show a pipeline by ID using the default organization
azdo pipelines show Fabrikam/42

# Show a pipeline by name
azdo pipelines show Fabrikam/My Pipeline

# Show with explicit organization
azdo pipelines show MyOrg/Fabrikam/42

# Export as JSON
azdo pipelines show Fabrikam/42 --json id,name,revision
```

### See also

* [azdo pipelines](./azdo_pipelines.md)
