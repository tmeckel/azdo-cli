## Command `azdo pipelines list`

```
azdo pipelines list [ORGANIZATION/]PROJECT [flags]
```

List pipeline definitions (YAML or classic) in a project.


### Options


* `--folder-path` `string`

	Filter by folder path (e.g. &#34;user1/production&#34;)

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--max-items` `int` (default `0`)

	Optional client-side cap on results

* `--name` `string`

	Filter by pipeline name (prefix or exact)

* `--query-order` `string`

	Order of definitions: {none|definitionNameAscending|definitionNameDescending|lastModifiedAscending|lastModifiedDescending}

* `--repository` `string`

	Filter by repository name or ID

* `--repository-type` `string`

	Repository type filter: {tfsgit|github}

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--top` `int` (default `0`)

	Maximum number of definitions to return


### ALIASES

- `ls`
- `l`

### JSON Fields

`_links`, `authoredBy`, `createdDate`, `draftOf`, `drafts`, `id`, `latestBuild`, `latestCompletedBuild`, `metrics`, `name`, `path`, `project`, `quality`, `queue`, `queueStatus`, `revision`, `type`, `uri`, `url`

### Examples

```bash
# List all pipelines in a project
$ azdo pipelines list "my-project"

# List pipelines with a specific name
$ azdo pipelines list "my-project" --name "my-pipeline"

# List pipelines using a specific repository
$ azdo pipelines list "my-project" --repository "my-repo"

# Output as JSON
$ azdo pipelines list "my-project" --json
```

### See also

* [azdo pipelines](./azdo_pipelines.md)
