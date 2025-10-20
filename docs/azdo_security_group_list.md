## Command `azdo security group list`

```
azdo security group list [ORGANIZATION[/PROJECT]] [flags]
```

List all security groups within a given project or organization.

### Options


* `-f`, `--filter` `string`

	Case-insensitive regex to filter groups by name (e.g. &#39;dev.*team&#39;). Invalid patterns will result in an error

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--subject-types` `strings`

	List of subject types to include (comma-separated). Values must not be empty or contain only whitespace.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`description`, `id`, `mailAddress`, `name`, `principalName`

### Examples

```bash
# List all security groups in the default organization
azdo security group list

# List all groups matching a regex pattern
azdo security group list --filter 'dev.*team'

# List groups filtering by regex and restricting by subject types
azdo security group list --filter '-qa$' --subject-types vssgp,aadgp
```

### See also

* [azdo security group](./azdo_security_group.md)
