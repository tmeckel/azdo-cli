## Command `azdo service-endpoint export`

```
azdo service-endpoint export [ORGANIZATION/]PROJECT/ID_OR_NAME [flags]
```

Export an Azure DevOps service endpoint (service connection) into a JSON definition.

The positional argument accepts the form [ORGANIZATION/]PROJECT/ID_OR_NAME. When the
organization segment is omitted the default organization from configuration is used.

### Options


* `-o`, `--output-file` `string`

	Path to write the exported JSON. Defaults to stdout.

* `--with-secrets`

	Include sensitive authorization values in the export.


### ALIASES

- `e`
- `ex`

### Examples

```bash
# Export to stdout with secrets redacted
azdo service-endpoint export my-org/MyProject/MyEndpoint

# Export to a file while including secrets
azdo service-endpoint export MyProject/058bff6f-2717-4500-af7e-3fffc2b0b546 --output-file ./endpoint.json --with-secrets
```

### See also

* [azdo service-endpoint](./azdo_service-endpoint.md)
