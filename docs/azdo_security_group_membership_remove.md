## Command `azdo security group membership remove`

```
azdo security group membership remove [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP [flags]
```

Remove a user or group from an Azure DevOps security group.

The positional argument accepts either ORGANIZATION/GROUP or ORGANIZATION/PROJECT/GROUP.
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
azdo security group membership remove MyOrg/Project Administrators --member user@example.com
```

### See also

* [azdo security group membership](./azdo_security_group_membership.md)
