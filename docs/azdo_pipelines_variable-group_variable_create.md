## Command `azdo pipelines variable-group variable create`

```
azdo pipelines variable-group variable create [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME --name VARIABLE_NAME [flags]
```

Add a variable to an existing variable group. Secret values are write-only and will be redacted in
human output and omitted from JSON output.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `string`

	Variable name to add (case-insensitive)

* `--prompt-value`

	Prompt securely for a secret value (only valid with --secret)

* `--read-only`

	Set the variable read-only

* `--secret`

	Mark the variable as secret (write-only)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--value` `string`

	Literal value for the variable


### JSON Fields

`name`, `readOnly`, `secret`, `value`

### See also

* [azdo pipelines variable-group variable](./azdo_pipelines_variable-group_variable.md)
