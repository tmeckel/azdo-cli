## azdo security permission namespace list
```
azdo security permission namespace list [ORGANIZATION] [flags]
```
List all security permission namespaces available in an Azure DevOps organization.

Namespaces define the scope and structure for security permissions on various resources.

### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields

* `--local-only`

	Only include namespaces defined locally within the organization.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### See also

* [azdo security permission namespace](./azdo_security_permission_namespace.md)
