## Command `azdo service-endpoint show`

```
azdo service-endpoint show [ORGANIZATION/]PROJECT/ID_OR_NAME [flags]
```

Show details of a single Azure DevOps service endpoint (service connection).

The positional argument accepts the form [ORGANIZATION/]PROJECT/ID_OR_NAME. When the
organization segment is omitted the default organization from configuration is used.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### JSON Fields

`administratorsGroup`, `authorization`, `createdBy`, `data`, `description`, `groupScopeId`, `id`, `isReady`, `isShared`, `name`, `operationStatus`, `owner`, `readersGroup`, `serviceEndpointProjectReferences`, `type`, `url`

### Examples

```bash
# Show a service endpoint by ID in the default organization
azdo service-endpoint show MyProject/12345678-1234-1234-1234-1234567890ab

# Show a service endpoint by name in a specific organization
azdo service-endpoint show myorg/MyProject/MyConnection
```

### See also

* [azdo service-endpoint](./azdo_service-endpoint.md)
