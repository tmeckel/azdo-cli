## Command `azdo security group show`

```
azdo security group show ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP [flags]
```

Display the details of an Azure DevOps security group within an organization or project scope.

The organization segment is required. Provide an optional project segment to narrow the search scope.


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

`description`, `descriptor`, `domain`, `mailAddress`, `name`, `origin`, `originId`, `principalName`, `url`

### Examples

```bash
# Show an organization-level security group
azdo security group show MyOrg/Project Collection Administrators

# Show a project-level security group
azdo security group show MyOrg/MyProject/Contributors

# Show details as JSON
azdo security group show MyOrg/Contributors --json
```

### See also

* [azdo security group](./azdo_security_group.md)
