## Command `azdo pipelines variable-group update`

```
azdo pipelines variable-group update [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME [flags]
```

Update a variable group's metadata (name, description, type, providerData),
manage cross-project sharing, and optionally toggle 'authorize for all pipelines'.


### Options


* `--authorize`

	Grant (true) or remove (false) access permission to all pipelines

* `--clear-project-references`

	Overwrite existing project references with the provided set; when provided without any --project-reference, removes all references

* `--clear-provider-data`

	Clear providerData (mutually exclusive with --provider-data-json)

* `--description` `string`

	New description (empty string clears it)

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `string`

	New display name

* `--project-reference` `stringArray`

	Project reference to share with (repeatable)

* `--provider-data-json` `string`

	Raw JSON payload for providerData

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--type` `string`

	Variable group type (e.g., Vsts, AzureKeyVault)


### JSON Fields

`createdBy`, `createdOn`, `description`, `id`, `isShared`, `modifiedBy`, `modifiedOn`, `name`, `pipelinePermissions`, `providerData`, `type`, `variableGroupProjectReferences`, `variables`

### See also

* [azdo pipelines variable-group](./azdo_pipelines_variable-group.md)
