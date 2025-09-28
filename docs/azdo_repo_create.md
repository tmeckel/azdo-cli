## azdo repo create
Create a new repository in a project
```
azdo repo create [ORGANIZATION/]<PROJECT>/<NAME> [flags]
```
### Options


* `--format` `string`

	Output format: {json}

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields

* `--parent` `string`

	[PROJECT/]REPO to fork from (same organization)

* `--source-branch` `string`

	Only fork the specified branch (defaults to all branches)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### Examples

```bash
# create a repository in specified project (org from default config)
azdo repo create myproject/myrepo

# create a repository in specified org/project
azdo repo create myorg/myproject/myrepo

# create a fork of an existing repo in another project
azdo repo create myproject/myfork --parent otherproject/otherrepo
```

### See also

* [azdo repo](./azdo_repo.md)
