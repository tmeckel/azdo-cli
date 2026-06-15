## Command `azdo pipelines run`

```
azdo pipelines run [ORGANIZATION/]PROJECT/PIPELINE [flags]
```

Queue (run) an existing Azure Pipeline definition. The pipeline is
resolved by positive numeric ID or by name.  Supply --branch,
--commit-id, and --variable to customise the run.


### Options


* `--branch` `string`

	Branch or ref to build (bare names get refs/heads/ prepended)

* `--commit-id` `string`

	Source commit SHA to build

* `--folder-path` `string`

	Folder path filter used when resolving a pipeline name

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--variable` `strings`

	Queue-time variable in name=value format (repeatable)


### JSON Fields

`buildNumber`, `id`, `queueTime`, `reason`, `result`, `sourceBranch`, `sourceVersion`, `status`

### Examples

```bash
# Queue a run by pipeline ID
azdo pipelines run Fabrikam/42

# Queue against a specific branch
azdo pipelines run MyOrg/Fabrikam/42 --branch main

# Queue with a commit and a variable
azdo pipelines run Fabrikam/MyPipeline --commit-id abc123 --variable env=prod
```

### See also

* [azdo pipelines](./azdo_pipelines.md)
