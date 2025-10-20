## Command `azdo security permission namespace show`

```
azdo security permission namespace show [ORGANIZATION/]NAMESPACE [flags]
```

Show the full details of a security permission namespace, including the actions it defines.

The namespace can be specified by its GUID or by its name. When using a name, the command performs
a case-insensitive match against both the namespace's name and display name.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `s`

### JSON Fields

`actions`, `actionsCount`, `dataspaceCategory`, `displayName`, `elementLength`, `extensionType`, `isRemotable`, `name`, `namespaceId`, `readPermission`, `separatorValue`, `structureValue`, `systemBitMask`, `useTokenTranslator`, `writePermission`

### Examples

```bash
# Show a namespace by ID using the default organization
azdo security permission namespace show 52d39943-cb85-4d7f-8fa8-c6baac873819

# Show a namespace by name using an explicit organization
azdo security permission namespace show myorg/Project Collection

# Display selected fields from the namespace as JSON
azdo security permission namespace show myorg/Build --json namespaceId,name,actions
```

### See also

* [azdo security permission namespace](./azdo_security_permission_namespace.md)
