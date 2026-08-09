## Command `azdo security group membership remove`

```
azdo security group membership remove [ORG:][PROJECT/]GROUP [flags]
```

Remove a user or group from an Azure DevOps security group.

The positional argument accepts [ORG:][PROJECT/]GROUP.
When the ORG: prefix is omitted the default organization from configuration is used.
Use --member to provide the member's email, descriptor, or principal name.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-m`, `--member` `strings`

	List of (comma-separated) Email, descriptor, or principal name of the user or group to remove.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-y`, `--yes`

	Do not prompt for confirmation.


### ALIASES

- `d`
- `r`
- `rm`
- `delete`
- `del`

### JSON Fields

`groupDescriptor`, `groupDisplayName`, `memberDescriptor`, `memberDisplayName`, `memberSubjectKind`, `relationshipRemoved`, `status`

### Examples

```bash
# Remove a user by email from an organization-level group
azdo security group membership remove /Project Administrators --member user@example.com
```

### See also

* [azdo security group membership](./azdo_security_group_membership.md)
