## Command `azdo auth login`

```
azdo auth login [flags]
```

Authenticate with a Azure DevOps Organization.

The default authentication mode is a an interactive process in which a personal access token (PAT) can be added. After completion, the
authentication token will be stored internally.

The minimum required scopes for the token are: `Code: Read`, `Project and Team: Read`

Using a web-based browser authentication flow, which creates a new PAT, is currently not supported.

Alternatively, use `--with-token` to pass in a token on standard input.

Alternatively, azdo will use the authentication token (PAT) found in environment variables.
This method is most suitable for "headless" use of azdo such as in automation. See
`azdo help environment` for more info.

To use azdo in Azure DevOps Pipeline Tasks (or other automation environments), add `AZDO_TOKEN: ${{ azdo.token }}` to "env".


### Options


* `-p`, `--git-protocol` `string`

	The protocol to use for git operations: {ssh|https}

* `--insecure-storage`

	Save authentication credentials in plain text instead of credential store

* `-o`, `--organization-url` `string`

	The URL to the Azure DevOps organization to authenticate with

* `--with-token`

	Read token from standard input


### Examples

```bash
# start interactive setup
$ azdo auth login

# authenticate by reading the token from a file
$ azdo auth login --with-token < mytoken.txt

# authenticate with a specific Azure DevOps Organization
$ azdo auth login --organizationUrl https://dev.azure.com/myorg
```

### See also

* [azdo auth](./azdo_auth.md)
