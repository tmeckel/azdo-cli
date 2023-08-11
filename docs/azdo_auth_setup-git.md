## azdo auth setup-git
```
azdo auth setup-git [flags]
```
This command configures git to use AzDO CLI as a credential helper.
For more information on git credential helpers please reference:
https://git-scm.com/docs/gitcredentials.

By default, AzDO CLI will be set as the credential helper for all authenticated organizations.
If there is no authenticated organization the command fails with an error.

Alternatively, use the `--organization` flag to specify a single organization to be configured.
If the organization is not authenticated with, the command fails with an error.

Be aware that a credential helper will only work with git remotes that use the HTTPS protocol.

### Options


* `-o`, `--organization` `string`

	Configure git credential helper for specific organization


### Examples

```bash
# Configure git to use AzDO CLI as the credential helper for all authenticated organizations
$ azdo auth setup-git

# Configure git to use AzDO CLI as the credential helper for a specific organization
$ azdo auth setup-git --organization enterprise.internal
```

### See also

* [azdo auth](./azdo_auth)
