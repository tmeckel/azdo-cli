## Command `azdo pipelines agent show`

```
azdo pipelines agent show [ORGANIZATION/]POOL/AGENT [flags]
```

Display the details of a single Azure DevOps pipeline agent.
The agent is specified as a pool and agent ID or name, with
an optional organization prefix.


### Options


* `--include-capabilities`

	Include system and user capabilities in the output

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-r`, `--raw`

	Dump raw agent object to stderr

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `view`
- `status`

### JSON Fields

`_links`, `accessPoint`, `assignedRequest`, `authorization`, `createdBy`, `createdOn`, `enabled`, `id`, `lastCompletedRequest`, `maxParallelism`, `name`, `osDescription`, `pendingUpdate`, `pool`, `properties`, `provisioningState`, `status`, `statusChangedOn`, `systemCapabilities`, `userCapabilities`, `version`

### Examples

```bash
# Show an agent by pool ID and agent ID
azdo pipelines agent show 1/42

# Show an agent by pool name and agent name
azdo pipelines agent show 'Default/my-agent'

# Show an agent in a specific organization
azdo pipelines agent show 'myorg/Default/my-agent'

# Show an agent with capabilities
azdo pipelines agent show 1/42 --include-capabilities

# Show agent as JSON
azdo pipelines agent show 1/42 --json
```

### See also

* [azdo pipelines agent](./azdo_pipelines_agent.md)
