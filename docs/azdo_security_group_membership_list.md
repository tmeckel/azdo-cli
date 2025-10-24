## Command `azdo security group membership list`

List the members of an Azure DevOps security group.

```
azdo security group membership list [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP [flags]
```

### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-r`, `--relationship` `string` (default `&#34;members&#34;`)

	Relationship type: members or memberof: {members|memberof}

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`descriptor`, `displayName`, `legacyDescriptor`, `origin`, `originId`, `subjectKind`, `url`

### See also

* [azdo security group membership](./azdo_security_group_membership.md)
