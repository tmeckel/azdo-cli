## Command `azdo pipelines build list`

```
azdo pipelines build list [ORGANIZATION/]PROJECT [flags]
```

List classic build (Build v1) records in a project. Supports filter,
pagination, and JSON export. For the modern Pipelines runs surface,
see 'azdo pipelines runs list'.


### Options


* `--branch` `string`

	Limit to builds for this branch. Bare names get refs/heads/ prepended.

* `--build-number` `string`

	Limit to builds that match this build number. Append * for prefix search.

* `--definition-id` `ints`

	Limit to builds for these definition IDs (repeatable)

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Maximum number of builds to return across all pages (client-side; 0 = unlimited)

* `--reason` `string`

	Limit to builds with this reason: {all|batchedci|buildcompletion|checkinshelveset|individualci|manual|none|pullrequest|resourcetrigger|schedule|scheduleforced|triggered|usercreated|validateshelveset}

* `--requested-for` `string`

	Limit to builds requested for this user or group; supports @me

* `--result` `string`

	Limit to builds with this result: {canceled|failed|none|partiallysucceeded|succeeded}

* `--status` `string`

	Limit to builds with this status: {all|cancelling|completed|inprogress|none|notstarted|postponed}

* `--tag` `strings`

	Limit to builds that have all of the specified tags (repeatable)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--top` `int` (default `0`)

	Maximum number of builds to return per page (server-side; 0 = server default)


### ALIASES

- `ls`
- `l`

### JSON Fields

`buildNumber`, `definition`, `finishTime`, `id`, `project`, `queueTime`, `reason`, `requestedBy`, `requestedFor`, `result`, `sourceBranch`, `sourceVersion`, `startTime`, `status`, `tags`, `uri`, `url`

### Examples

```bash
# List the 20 most recent builds for a project
azdo pipelines build list Fabrikam --top 20

# Filter by branch, status, and tag
azdo pipelines build list Fabrikam --branch main --status completed --tag release

# Export as JSON
azdo pipelines build list Fabrikam --json id,buildNumber,status,result
```

### See also

* [azdo pipelines build](./azdo_pipelines_build.md)
