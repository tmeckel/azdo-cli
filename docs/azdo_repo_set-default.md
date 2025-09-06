## azdo repo set-default
```
azdo repo set-default [<repository>] [flags]
```

			This command sets the default remote repository to use when querying the
			Azure DevOps API for the locally cloned repository.

			azdo uses the default repository for things like:

			- viewing and creating pull requests
			- viewing and creating issues
			- viewing and creating releases
			- working with Azure DevOps Pipelines

			The command will only take configured remotes into account which point to a Azure DevOps organization.
### Options


* `-u`, `--unset`

	unset the current default repository

* `-v`, `--view`

	view the current default repository


### Examples

```bash
Interactively select a default repository:
$ azdo repo set-default

Set a repository explicitly:
$ azdo repo set-default [organization/]project/repo

View the current default repository:
$ azdo repo set-default --view

Show more repository options in the interactive picker:
$ git remote add newrepo https://dev.azure.com/myorg/myrepo/_git/myrepo
$ azdo repo set-default
```

### See also

* [azdo repo](./azdo_repo.md)
