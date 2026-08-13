## Command `azdo pipelines folder create`

```
azdo pipelines folder create [ORG:]PROJECT/PATH [flags]
```

Create a build definition folder at PATH under PROJECT.

Mirrors 'az pipelines folder create'. PATH is the full path
(e.g. "External/CI"). Azure DevOps stores folder paths with '/'.


### Options


* `--description` `string`

	Description of the folder.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `c`
- `cr`

### JSON Fields

`createdBy`, `createdOn`, `description`, `lastChangedBy`, `lastChangedDate`, `path`, `project`

### Examples

```bash
# Create a folder in the default organization
azdo pipelines folder create Fabrikam/External/CI

# Create a folder in a specific organization
azdo pipelines folder create myorg:Fabrikam/External/CI

# Create a folder with a description
azdo pipelines folder create Fabrikam/External/CI --description "CI folders"

# Output as JSON
azdo pipelines folder create Fabrikam/External/CI --json
```

### See also

* [azdo pipelines folder](./azdo_pipelines_folder.md)
