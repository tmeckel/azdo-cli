## Command `azdo repo delete`

Delete a Git repository in a team project

```
azdo repo delete [organization/]project/repository [flags]
```

### Options


* `-y`, `--yes`

	Do not prompt for confirmation


### ALIASES

- `d`

### Examples

```bash
# delete a repository in the default organization
azdo repo delete myproject/myrepo

# delete a repository using specified organization
azdo repo delete myorg/myproject/myrepo
```

### See also

* [azdo repo](./azdo_repo.md)
