## Command `azdo service-endpoint create`

```
azdo service-endpoint create [ORGANIZATION/]PROJECT --from-file <path> [flags]
```

Create Azure DevOps service endpoints (service connections) from a JSON definition file.

The project scope accepts the form [ORGANIZATION/]PROJECT. When the organization segment
is omitted the default organization from configuration is used.

Check the available subcommands to create service connections of specific well-known types.


### Available commands

* [azdo service-endpoint create azurerm](./azdo_service-endpoint_create_azurerm.md)
* [azdo service-endpoint create github](./azdo_service-endpoint_create_github.md)

### Options


* `-e`, `--encoding` `string` (default `&#34;utf-8&#34;`)

	File encoding (utf-8, ascii, utf-16be, utf-16le).

* `-f`, `--from-file` `string`

	Path to the JSON service endpoint definition or &#39;-&#39; for stdin.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `import`

### JSON Fields

`administratorsGroup`, `authorization`, `createdBy`, `data`, `description`, `groupScopeId`, `id`, `isReady`, `isShared`, `name`, `operationStatus`, `owner`, `readersGroup`, `serviceEndpointProjectReferences`, `type`, `url`

### Examples

```bash
# Create a service endpoint from a UTF-8 JSON file
azdo service-endpoint create my-org/my-project --from-file ./endpoint.json

# Read the definition from stdin using UTF-16LE encoding
cat endpoint.json | azdo service-endpoint create my-org/my-project --from-file - --encoding utf-16le
```

### See also

* [azdo service-endpoint](./azdo_service-endpoint.md)
