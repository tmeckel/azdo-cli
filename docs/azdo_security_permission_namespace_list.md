## Command `azdo security permission namespace list`

```
azdo security permission namespace list [ORGANIZATION] [flags]
```

List all security permission namespaces available in an Azure DevOps organization.

Namespaces define the scope and structure for security permissions on various resources.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--local-only`

	Only include namespaces defined locally within the organization.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`dataspaceCategory`, `displayName`, `elementLength`, `extensionType`, `isRemotable`, `name`, `namespaceId`, `readPermission`, `separatorValue`, `structureValue`, `systemBitMask`, `useTokenTranslator`, `writePermission`

### See also

* [azdo security permission namespace](./azdo_security_permission_namespace.md)
