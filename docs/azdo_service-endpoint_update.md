## Command `azdo service-endpoint update`

```
azdo service-endpoint update [ORGANIZATION/]PROJECT/ID_OR_NAME [flags]
```

Update an existing Azure DevOps service endpoint (service connection).

The positional argument accepts the form [ORGANIZATION/]PROJECT/ID_OR_NAME. When the
organization segment is omitted the default organization from configuration is used.

Provide one or more mutating flags to change attributes or pipeline permissions.


### Options


* `--description` `string`

	New description for the service endpoint.

* `--enable-for-all`

	Grant (true) or revoke (false) access for all pipelines.

* `-e`, `--encoding` `string` (default `&#34;utf-8&#34;`)

	File encoding (utf-8, ascii, utf-16be, utf-16le).

* `-f`, `--from-file` `string`

	Path to a JSON service endpoint definition or &#39;-&#39; for stdin. Mutually exclusive with --name/--description/--url.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `string`

	New friendly name for the service endpoint.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--url` `string`

	New service endpoint URL.


### JSON Fields

`administratorsGroup`, `authorization`, `createdBy`, `data`, `description`, `groupScopeId`, `id`, `isReady`, `isShared`, `name`, `operationStatus`, `owner`, `readersGroup`, `serviceEndpointProjectReferences`, `type`, `url`

### See also

* [azdo service-endpoint](./azdo_service-endpoint.md)
