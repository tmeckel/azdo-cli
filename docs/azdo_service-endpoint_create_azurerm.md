## Command `azdo service-endpoint create azurerm`

```
azdo service-endpoint create azurerm [ORGANIZATION/]PROJECT --name <name> --authentication-scheme <scheme> [flags]
```

Create an Azure Resource Manager service connection.
This command is modeled after the Azure DevOps Terraform Provider's implementation for creating azurerm service endpoints.


### Options


* `--authentication-scheme` `string` (default `&#34;ServicePrincipal&#34;`)

	Authentication scheme: {ServicePrincipal|ManagedServiceIdentity|WorkloadIdentityFederation}

* `--certificate-path` `string`

	Path to service principal certificate file (PEM format)

* `--description` `string`

	Description for the service endpoint

* `--environment` `string` (default `&#34;AzureCloud&#34;`)

	Azure environment: {AzureCloud|AzureChinaCloud|AzureUSGovernment|AzureGermanCloud|AzureStack}

* `--grant-permission-to-all-pipelines`

	Grant access permission to all pipelines to use the service connection

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--management-group-id` `string`

	Azure management group ID

* `--management-group-name` `string`

	Azure management group name

* `--name` `string`

	Name of the service endpoint

* `--resource-group` `string`

	Name of the resource group (for subscription-level scope)

* `--server-url` `string`

	Azure Stack Resource Manager base URL. Required if --environment is AzureStack.

* `--service-principal-id` `string`

	Service principal/application ID (e.g., GUID)

* `--service-principal-key` `string`

	Service principal key (secret value)

* `--subscription-id` `string`

	Azure subscription ID (e.g., GUID)

* `--subscription-name` `string`

	Azure subscription name

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--tenant-id` `string`

	Azure tenant ID (e.g., GUID)

* `-y`, `--yes`

	Skip confirmation prompts


### ALIASES

- `cr`
- `c`
- `new`
- `n`
- `add`
- `a`

### JSON Fields

`administratorsGroup`, `authorization`, `createdBy`, `data`, `description`, `groupScopeId`, `id`, `isReady`, `isShared`, `name`, `operationStatus`, `owner`, `readersGroup`, `serviceEndpointProjectReferences`, `type`, `url`

### Examples

```bash
# Service Principal with a secret
azdo service-endpoint create azurerm my-org/my-project \
		--name "My AzureRM SPN Secret Connection" \
		--authentication-scheme ServicePrincipal \
		--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
		--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
		--service-principal-key "my-service-principal-secret" \
		--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
		--subscription-name "My Azure Subscription" \
		--resource-group "my-resource-group" \
		--description "Service Connection for my AzureRM resources"

# Service Principal with a certificate
azdo service-endpoint create azurerm my-org/my-project \
		--name "My AzureRM SPN Cert Connection" \
		--authentication-scheme ServicePrincipal \
		--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
		--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
		--certificate-path "/path/to/my-cert.pem" \
		--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
		--subscription-name "My Azure Subscription" \
		--description "Certificate-based Service Connection"

# Managed Service Identity
azdo service-endpoint create azurerm my-org/my-project \
		--name "My AzureRM MSI Connection" \
		--authentication-scheme ManagedServiceIdentity \
		--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
		--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
		--subscription-name "My Azure Subscription" \
		--description "MSI Service Connection"

# Workload Identity Federation (Manual mode, with existing Service Principal)
azdo service-endpoint create azurerm my-org/my-project \
		--name "My AzureRM WIF Manual Connection" \
		--authentication-scheme WorkloadIdentityFederation \
		--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
		--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
		--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
		--subscription-name "My Azure Subscription" \
		--description "WIF Manual Service Connection"

# Workload Identity Federation (Automatic mode, Azure DevOps creates Service Principal)
azdo service-endpoint create azurerm my-org/my-project \
		--name "My AzureRM WIF Automatic Connection" \
		--authentication-scheme WorkloadIdentityFederation \
		--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
		--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
		--subscription-name "My Azure Subscription" \
		--description "WIF Automatic Service Connection"

# Service Principal with Management Group Scope
azdo service-endpoint create azurerm my-org/my-project \
		--name "My AzureRM MGMT Group Connection" \
		--authentication-scheme ServicePrincipal \
		--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
		--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
		--service-principal-key "my-service-principal-secret" \
		--management-group-id "my-mgmt-group-id" \
		--management-group-name "My Management Group" \
		--description "Service Connection scoped to a Management Group"

# Azure Stack Environment
azdo service-endpoint create azurerm my-org/my-project \
		--name "My AzureStack Connection" \
		--authentication-scheme ServicePrincipal \
		--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
		--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
		--service-principal-key "my-service-principal-secret" \
		--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
		--subscription-name "My Azure Stack Subscription" \
		--environment AzureStack \
		--server-url "https://management.myazurestack.com/" \
		--description "Service Connection for Azure Stack"
```

### See also

* [azdo service-endpoint create](./azdo_service-endpoint_create.md)
