## Command `azdo pipelines runs list`

```
azdo pipelines runs list [ORGANIZATION/]PROJECT [flags]
```

List runs of pipelines in an Azure DevOps project. Mirrors
'az pipelines runs list'.

Filters support pipeline, branch, status, result, reason, requester,
and tags. The full result set is paginated server-side; use
--max-items to cap the response client-side.


### Options


* `--branch` `string`

	Filter by source branch. Bare names get refs/heads/ prepended.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Maximum number of runs to return client-side (0 = unlimited).

* `--pipeline-id` `ints`

	Filter by pipeline IDs (repeatable).

* `--query-order` `string`

	Order the results: {finishtimeascending|finishtimedescending|queuetimeascending|queuetimedescending|starttimeascending|starttimedescending}

* `--reason` `string`

	Filter by reason: {all|batchedci|buildcompletion|checkinshelveset|individualci|manual|none|pullrequest|resourcetrigger|schedule|scheduleforced|triggered|usercreated|validateshelveset}

* `--requested-for` `string`

	Filter by the user who queued the run. Accepts @me to mean the authenticated user.

* `--result` `string`

	Filter by result: {canceled|failed|none|partiallysucceeded|succeeded}

* `--status` `string`

	Filter by status: {all|cancelling|completed|inprogress|none|notstarted|postponed}

* `--tag` `strings`

	Filter by tags (all supplied tags must match).

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--top` `int` (default `0`)

	Maximum number of runs to request per server page (0 = server default).


### ALIASES

- `l`
- `ls`

### JSON Fields

`buildNumber`, `definition`, `finishTime`, `id`, `project`, `queueTime`, `reason`, `requestedBy`, `requestedFor`, `result`, `sourceBranch`, `sourceVersion`, `startTime`, `status`, `tags`, `uri`, `url`

### Examples

```bash
# List the 20 most recent runs for a project (default org)
azdo pipelines runs list Fabrikam --top 20

# Filter by pipeline and branch
azdo pipelines runs list MyOrg/Fabrikam --pipeline-id 42 --branch main

# Order by queue time, descending
azdo pipelines runs list Fabrikam --query-order queueTimeDescending

# Export as JSON
azdo pipelines runs list Fabrikam --json id,buildNumber,status,result
```

### See also

* [azdo pipelines runs](./azdo_pipelines_runs.md)
