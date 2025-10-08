## azdo security group create
```
azdo security group create [ORGANIZATION|ORGANIZATION/PROJECT] [flags]
```
Create a security group in an Azure DevOps organization or project.

Security groups can be created by name, email, or origin ID. Exactly one of these must be specified.

### Options


* `--description` `string`

	Description of the new security group.

* `--email` `string`

	Create a security group using an existing AAD group&#39;s email address.

* `--groups` `strings`

	A comma-separated list of group descriptors to add the new group to.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields

* `--name` `string`

	Name of the new security group.

* `--origin-id` `string`

	Create a security group using an existing AAD group&#39;s origin ID.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### See also

* [azdo security group](./azdo_security_group.md)
