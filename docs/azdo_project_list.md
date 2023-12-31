## azdo project list
List the projects for an organization
```
azdo project list [flags]
```
### Options


* `--format` `string`

	Output format: {json}

* `-l`, `--limit` `int`

	Maximum number of projects to fetch

* `-o`, `--organization` `string`

	Get per-organization configuration

* `--state` `string`

	Project state filter: {deleting|new|wellFormed|createPending|all|unchanged|deleted}


### Examples

```bash
# list the default organizations's projects
azdo project list

# list the projects for an Azure DevOps organization including closed projects
azdo project list --organization myorg --closed
```

### See also

* [azdo project](./azdo_project.md)
