## Command `azdo pipelines variable-group show`

```
azdo pipelines variable-group show [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME [flags]
```

Display metadata for a variable group, including its authorization state and variables.

The positional argument accepts the form [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME.
When the organization segment is omitted the default organization from configuration is used.


### Options


* `--include-project-references`

	Include variableGroupProjectReferences details

* `--include-provider-data`

	Include providerData payloads

* `--include-variables`

	Include variable values (secrets remain redacted)

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### JSON Fields

`createdBy`, `createdOn`, `description`, `id`, `isShared`, `modifiedBy`, `modifiedOn`, `name`, `pipelinePermissions`, `providerData`, `type`, `variableGroupProjectReferences`, `variables`

### See also

* [azdo pipelines variable-group](./azdo_pipelines_variable-group.md)
