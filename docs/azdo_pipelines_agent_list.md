## Command `azdo pipelines agent list`

```
azdo pipelines agent list [ORGANIZATION/]POOL [flags]
```

List every agent in an Azure DevOps agent pool.
The pool is identified by a positional target that can be a numeric ID or a name.


### Options


* `-f`, `--filter` `string`

	Filter agents by name

* `--include-capabilities`

	Include agent capabilities in the response

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Optional client-side cap on results

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`_links`, `accessPoint`, `assignedAgentCloudRequest`, `assignedRequest`, `authorization`, `createdOn`, `enabled`, `id`, `lastCompletedRequest`, `maxParallelism`, `name`, `osDescription`, `pendingUpdate`, `properties`, `provisioningState`, `status`, `statusChangedOn`, `systemCapabilities`, `userCapabilities`, `version`

### Examples

```bash
# List all agents in pool 1
$ azdo pipelines agent list 1

# List agents in a named pool
$ azdo pipelines agent list Default

# List agents in pool 1 in a specific organization
$ azdo pipelines agent list "myorg/1"

# List agents in a named pool in a specific organization
$ azdo pipelines agent list "myorg/Default"

# List agents filtered by name
$ azdo pipelines agent list 1 --filter "my-agent"

# List agents filtered by name in a specific organization
$ azdo pipelines agent list "myorg/1" --filter "my-agent"

# List agents with capabilities included
$ azdo pipelines agent list 1 --include-capabilities

# Output as JSON
$ azdo pipelines agent list 1 --json
```

### See also

* [azdo pipelines agent](./azdo_pipelines_agent.md)
