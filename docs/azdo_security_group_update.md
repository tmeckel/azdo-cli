## Command `azdo security group update`

```
azdo security group update ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP [flags]
```

Update the display name and/or description of an Azure DevOps security group.

Provide the organization segment and optional project segment to scope the lookup. At least one of --name or --description must be specified.


### Options


* `--description` `string`

	New description for the security group.

* `--descriptor` `string`

	Descriptor of the security group (required if multiple groups match the name).

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `string`

	New display name for the security group.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `u`

### JSON Fields

`description`, `descriptor`, `domain`, `mailAddress`, `name`, `origin`, `originId`, `principalName`, `url`

### Examples

```bash
# Update only the description of a project-level group
azdo security group update MyOrg/MyProject/Developers --description "Updated description"

# Update the name of an organization-level group
azdo security group update MyOrg/Old Group Name --name "New Group Name"
```

### See also

* [azdo security group](./azdo_security_group.md)
