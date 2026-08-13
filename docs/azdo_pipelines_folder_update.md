## Command `azdo pipelines folder update`

```
azdo pipelines folder update [ORG:]PROJECT/PATH [flags]
```

Update the path or description of a build definition folder.

Mirrors 'az pipelines folder update'. At least one of --new-path or
--new-description must be specified. The full updated folder is sent
to the server (full replace, not a partial patch).


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--new-description` `string`

	New description for the folder.

* `--new-path` `string`

	New full path for the folder.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `u`

### JSON Fields

`createdBy`, `createdOn`, `description`, `lastChangedBy`, `lastChangedDate`, `path`, `project`

### Examples

```bash
# Rename a folder in the default organization
azdo pipelines folder update Fabrikam/External/CI --new-path Fabrikam/External/Release

# Change only the description
azdo pipelines folder update myorg:Fabrikam/External/CI --new-description "Release pipeline folder"

# Rename and re-describe, output as JSON
azdo pipelines folder update Fabrikam/External/CI --new-path Fabrikam/External/Release --new-description "Release pipelines" --json
```

### See also

* [azdo pipelines folder](./azdo_pipelines_folder.md)
