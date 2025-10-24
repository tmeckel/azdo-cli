## Command `azdo repo list`

List repositories of a project inside an organization

```
azdo repo list [organization/]<project> [flags]
```

### Options


* `--format` `string` (default `&#34;table&#34;`)

	Output format: {json}

* `--include-hidden`

	Include hidden repositories

* `-L`, `--limit` `int` (default `30`)

	Maximum number of repositories to list

* `--visibility` `string`

	Filter by repository visibility: {public|private}


### ALIASES

- `ls`
- `l`

### Examples

```bash
# list the repositories of a project using default organization
azdo repo list myproject

# list the repositories of a project using specified organization
azdo repo list myorg/myproject
```

### See also

* [azdo repo](./azdo_repo.md)
