## Command `azdo pipelines variable-group variable update`

```
azdo pipelines variable-group variable update [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME --name VARIABLE_NAME [flags]
```

Update an existing variable in a variable group. Supports renaming, value changes,
toggling secret/read-only flags, prompting for secret values, and applying changes
from JSON. Secret values are write-only and will be redacted in human output and
omitted from JSON output.


### Options


* `--clear-value`

	Clear the stored value for a non-secret variable (destructive)

* `--from-json` `string`

	Apply updates from JSON (file path, &#39;-&#39;, or inline JSON)

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `string`

	Variable name to update (case-insensitive)

* `--new-name` `string`

	Rename the variable (case-insensitive)

* `--prompt-value`

	Prompt securely for a secret value (write-only)

* `--read-only`

	Set variable read-only (tri-state: only when explicitly set)

* `--secret`

	Set variable as secret (tri-state: only when explicitly set)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--value` `string`

	Replace the variable value

* `--yes`

	Skip confirmation prompts for destructive operations


### JSON Fields

`name`, `readOnly`, `secret`, `value`

### See also

* [azdo pipelines variable-group variable](./azdo_pipelines_variable-group_variable.md)
