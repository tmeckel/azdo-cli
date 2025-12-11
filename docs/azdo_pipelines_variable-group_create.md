## Command `azdo pipelines variable-group create`

```
azdo pipelines variable-group create [ORGANIZATION/]PROJECT/NAME [flags]
```

Create a variable group in a project, optionally seeding variables, linking Key Vault secrets,
authorizing access for all pipelines, and sharing the group across additional projects.


### Options


* `-A`, `--authorize`

	Authorize the variable group for all pipelines in the project after creation

* `-d`, `--description` `string`

	Optional description for the variable group

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--keyvault-name` `string`

	Azure Key Vault name backing the variable group

* `--keyvault-secret` `strings`

	Map a pipeline variable to a Key Vault secret (variable=secretName); repeat for multiple entries

* `--keyvault-service-endpoint` `string`

	Service endpoint ID (UUID) or name, that grants access to the Azure Key Vault

* `--project-reference` `strings`

	Additional project names or IDs to share the group with (repeat or comma-separate)

* `--provider-data-json` `string`

	Raw JSON payload for providerData (advanced; cannot be combined with Key Vault options)

* `--secret` `strings`

	Seed secret variables using key[=value]; value falls back to AZDO_PIPELINES_SECRET_&lt;NAME&gt; or an interactive prompt

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--type` `string` (default `&#34;Vsts&#34;`)

	Variable group type: {Vsts|AzureKeyVault}

* `-v`, `--variable` `strings`

	Seed non-secret variables using key=value[;readOnly=true|false]


### JSON Fields

`createdBy`, `createdOn`, `description`, `id`, `isShared`, `modifiedBy`, `modifiedOn`, `name`, `providerData`, `type`, `variableGroupProjectReferences`, `variables`

### Examples

```bash
# Create a generic variable group with seeded variables
azdo pipelines variable-group create MyOrg/MyProject/SharedConfig \
	--description "Shared non-secret settings" \
	--variable env=prod --variable region=westus;readOnly=true

# Create a Key Vault-backed variable group authorized for all pipelines
azdo pipelines variable-group create MyProject/SensitiveSecrets \
	--keyvault-service-endpoint 11111111-2222-3333-4444-555555555555 \
	--keyvault-name my-vault --keyvault-secret dbPassword=ProdDbSecret \
	--authorize

# Share a group with an additional project by name
azdo pipelines variable-group create MyOrg/ProjectA/CrossProject \
	--project-reference ProjectB
```

### See also

* [azdo pipelines variable-group](./azdo_pipelines_variable-group.md)
