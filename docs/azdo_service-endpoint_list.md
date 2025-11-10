## Command `azdo service-endpoint list`

```
azdo service-endpoint list [ORGANIZATION/]PROJECT [flags]
```

List Azure DevOps service endpoints (service connections) defined within a project.

The project scope accepts the form [ORGANIZATION/]PROJECT. When the organization
segment is omitted the default organization from configuration is used.


### Options


* `--action-filter` `string`

	Filter endpoints by caller permissions (manage, use, view, none).

* `--auth-scheme` `strings`

	Filter by authorization scheme. Repeat to specify multiple values or separate multiple values by comma &#39;,&#39;.

* `--endpoint-id` `strings`

	Filter by endpoint ID (UUID). Repeat to specify multiple values or separate multiple values by comma &#39;,&#39;.

* `--include-details`

	Request additional authorization metadata when available.

* `--include-failed`

	Include endpoints that are in a failed state.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `strings`

	Filter by endpoint display name. Repeat to specify multiple values or separate multiple values by comma &#39;,&#39;.

* `--output-format` `string` (default `&#34;table&#34;`)

	Select non-JSON output format: {table|ids}

* `--owner` `string`

	Filter by service endpoint owner (e.g., Library, AgentCloud).

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--type` `string`

	Filter by service endpoint type (e.g., AzureRM, GitHub, Generic).


### ALIASES

- `ls`
- `l`

### JSON Fields

`authorizationScheme`, `createdBy`, `id`, `isReady`, `isShared`, `name`, `owner`, `projectReference`, `type`

### Examples

```bash
# List service endpoints for a project using the default organization
azdo service-endpoint list MyProject

# List service endpoints for a project in a specific organization
azdo service-endpoint list myorg/MyProject

# List AzureRM endpoints that are ready for use
azdo service-endpoint list myorg/MyProject --type AzureRM --action-filter manage
```

### See also

* [azdo service-endpoint](./azdo_service-endpoint.md)
