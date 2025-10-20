## Command `azdo security group membership add`

```
azdo security group membership add [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP [flags]
```

Add a user or group as a member to an Azure DevOps security group.

The positional argument accepts either ORGANIZATION/GROUP or ORGANIZATION/PROJECT/GROUP.
Use --member to provide the member's email, descriptor, or principal name.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-m`, `--member` `strings`

	List of (comma-separated) Email, descriptor, or principal name of the user or group to add.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `a`
- `create`
- `cr`

### JSON Fields

`groupDescriptor`, `groupDisplayName`, `memberDescriptor`, `memberDisplayName`, `memberOrigin`, `memberOriginId`, `memberSubjectKind`, `relationshipCreated`, `status`

### Examples

```bash
# Add a user by email to an organization-level group
azdo security group membership add MyOrg/Project Administrators --member user@example.com

# Add a group by descriptor to a project-level group
azdo security group membership add MyOrg/MyProject/Readers --member vssgp.Uy0xLTItMw==
```

### See also

* [azdo security group membership](./azdo_security_group_membership.md)
