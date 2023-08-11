## azdo repo list
List repositories of a project inside an organization
```
azdo repo list <project> [flags]
```
### Options


* `--format` `string`

	Output format: {json}

* `--include-hidden`

	Include hidden repositories

* `-L`, `--limit` `int`

	Maximum number of repositories to list

* `-o`, `--organization` `string`

	Get per-organization configuration

* `--visibility` `string`

	Filter by repository visibility: {public|private}


### Examples

```bash
# list the repositories of a project using default organization
azdo repo list myproject

# list the repositories of a project using specified organization
azdo repo list myproject --organization myorg
```

### See also

* [azdo repo](./azdo_repo.md)
