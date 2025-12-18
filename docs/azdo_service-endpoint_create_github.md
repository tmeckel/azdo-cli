## Command `azdo service-endpoint create github`

```
azdo service-endpoint create github [ORGANIZATION/]PROJECT --name NAME [--url URL] [--token TOKEN] [flags]
```

Create a GitHub service endpoint using a personal access token (PAT) or an installation/oauth configuration.


### Options


* `--configuration-id` `string`

	Configuration for connecting to the endpoint (use an OAuth/installation configuration). Mutually exclusive with --token.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `string`

	Name of the service endpoint

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--token` `string`

	Visit https://github.com/settings/tokens to create personal access tokens. Recommended scopes: repo, user, admin:repo_hook. If omitted, you will be prompted for a token when interactive.

* `--url` `string`

	GitHub URL (defaults to https://github.com)


### JSON Fields

`administratorsGroup`, `authorization`, `createdBy`, `data`, `description`, `groupScopeId`, `id`, `isReady`, `isShared`, `name`, `operationStatus`, `owner`, `readersGroup`, `serviceEndpointProjectReferences`, `type`, `url`

### Examples

```bash
# Create a GitHub service endpoint with a personal access token (PAT)
azdo service-endpoint create github my-org/my-project --name "gh-ep" --token <PAT>

# Create a GitHub service endpoint with an installation / OAuth configuration id
azdo service-endpoint create github my-org/my-project --name "gh-ep" --configuration-id <CONFIG_ID>
```

### See also

* [azdo service-endpoint create](./azdo_service-endpoint_create.md)
