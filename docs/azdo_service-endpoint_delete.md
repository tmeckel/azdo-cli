## Command `azdo service-endpoint delete`

```
azdo service-endpoint delete [ORGANIZATION/]PROJECT/ID_OR_NAME [flags]
```

Delete an Azure DevOps service endpoint (service connection) from a project.

The positional argument accepts the form [ORGANIZATION/]PROJECT/ID_OR_NAME. When the
organization segment is omitted the default organization from configuration is used.


### Options


* `--additional-project` `stringArray`

	Additional project scope [ORGANIZATION/]PROJECT when the endpoint is shared. (Repeatable, comma-separated)

* `--deep`

	Also delete the backing Azure AD application for supported endpoints.

* `-y`, `--yes`

	Skip the confirmation prompt.


### ALIASES

- `rm`
- `del`
- `d`

### Examples

```bash
# Delete by endpoint ID inside the default organization
azdo service-endpoint delete MyProject/058bff6f-2717-4500-af7e-3fffc2b0b546

# Delete by name inside a specific organization, removing shares in another project
azdo service-endpoint delete myorg/MyProject/My Connection --additional-project myorg/SharedProject

# Deep delete an AzureRM connection and suppress the confirmation
azdo service-endpoint delete myorg/MyProject/ProdConnection --deep --yes
```

### See also

* [azdo service-endpoint](./azdo_service-endpoint.md)
