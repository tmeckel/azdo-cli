## Command `azdo repo show`

```
azdo repo show [ORGANIZATION/]PROJECT/REPO_ID_OR_NAME [flags]
```

Display the details of a single Azure DevOps Git repository.

The repository is identified by name or ID. The organization segment is optional when a
default organization is configured.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `view`
- `status`

### JSON Fields

`_links`, `defaultBranch`, `id`, `isDisabled`, `isFork`, `isInMaintenance`, `name`, `parentRepository`, `project`, `properties`, `remoteUrl`, `size`, `sshUrl`, `url`, `validRemoteUrls`, `webUrl`

### Examples

```bash
# Show a repository by name
azdo repo show Fabrikam/my-repo

# Show a repository by ID
azdo repo show myorg/Fabrikam/00000000-0000-0000-0000-000000000000
```

### See also

* [azdo repo](./azdo_repo.md)
