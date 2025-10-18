## azdo security group membership list
List the members of an Azure DevOps security group.
```
azdo security group membership list [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP [flags]
```
### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields

* `-r`, `--relationship` `string`

	Relationship type: members or memberof: {members|memberof}

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### See also

* [azdo security group membership](./azdo_security_group_membership.md)
