## azdo repo edit
```
azdo repo edit [organization/]project/repository [flags]
```
Modify properties of an Azure DevOps Git repository, including changing its default branch, renaming it, or toggling its disabled state.

Constraints for disabled repositories:
- When a repository is disabled, the only permitted action is to enable it using --enable.
- Attempts to change the default branch, rename, or disable an already-disabled repository will be blocked with a clear error message.
- Trying to re-disable a disabled repository or re-enable an enabled repository will also produce a specific "already disabled/enabled" error.

### Options


* `--default-branch` `string`

	Set the default branch for the repository

* `--disable`

	Disable the repository

* `--enable`

	Enable the repository

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields

* `--name` `string`

	Rename the repository

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### Examples

```bash
# Change the default branch (org from default config)
azdo repo edit myproject/myrepo --default-branch live

# Change the default branch with a full ref
azdo repo edit myorg/myproject/myrepo --default-branch refs/heads/live

# Rename a repository
azdo repo edit myproject/myrepo --name NewRepoName

# Disable a repository
azdo repo edit myproject/myrepo --disable

# Enable a previously disabled repository
azdo repo edit myproject/myrepo --enable

# Error: trying to disable an already disabled repo
azdo repo edit myproject/myrepo --disable

# Error: trying to make changes to a disabled repo (must enable first)
azdo repo edit myproject/myrepo --default-branch main
```

### See also

* [azdo repo](./azdo_repo.md)
