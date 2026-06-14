## Command `azdo pipelines runs show`

```
azdo pipelines runs show [ORGANIZATION/]PROJECT RUN_ID [flags]
```

Display the details of a single Azure Pipelines run.

Mirrors 'az pipelines runs show'.


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

`buildNumber`, `definition`, `finishTime`, `id`, `lastChangedBy`, `parameters`, `priority`, `queue`, `queueTime`, `reason`, `requestedBy`, `requestedFor`, `result`, `retainedByRelease`, `sourceBranch`, `sourceVersion`, `startTime`, `status`, `tags`, `triggerInfo`, `url`

### Examples

```bash
# Show a run by ID using the default organization
azdo pipelines runs show Fabrikam 12345

# Show a run by ID with explicit organization
azdo pipelines runs show MyOrg/Fabrikam 12345

# Export as JSON
azdo pipelines runs show Fabrikam 12345 --json id,buildNumber,status,result
```

### See also

* [azdo pipelines runs](./azdo_pipelines_runs.md)
