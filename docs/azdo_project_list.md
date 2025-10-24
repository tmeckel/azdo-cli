## Command `azdo project list`

List the projects for an organization

```
azdo project list [organization] [flags]
```

### Options


* `--format` `string` (default `&#34;table&#34;`)

	Output format: {json}

* `-l`, `--limit` `int` (default `30`)

	Maximum number of projects to fetch

* `--state` `string`

	Project state filter: {deleting|new|wellFormed|createPending|all|unchanged|deleted}


### ALIASES

- `ls`
- `l`

### Examples

```bash
# list the default organizations's projects
azdo project list

# list the projects for an Azure DevOps organization including closed projects
azdo project list --organization myorg --closed
```

### See also

* [azdo project](./azdo_project.md)
