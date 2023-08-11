## azdo auth logout
```
azdo auth logout [flags]
```
Remove authentication for a Azure DevOps organization.

This command removes the authentication configuration for an organization either specified
interactively or via `--organization`.

### Options


* `-o`, `--organization` `string`

	The Azure DevOps organization to log out of


### Examples

```bash
$ azdo auth logout
# => select what organization to log out of via a prompt

$ azdo auth logout --hostname enterprise.internal
# => log out of specified organization
```

### See also

* [azdo auth](./azdo_auth.md)
