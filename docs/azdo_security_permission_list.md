## Command `azdo security permission list`

```
azdo security permission list [TARGET] [flags]
```

List security access control entries (ACEs) for an Azure DevOps security namespace.

Accepted TARGET formats:
  - (empty)                        → use the default organization
  - ORGANIZATION                   → list all ACLs for the namespace in the organization
  - ORGANIZATION/SUBJECT           → list ACLs for the specified subject
  - ORGANIZATION/PROJECT/SUBJECT   → list ACLs for the subject scoped to the project


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-n`, `--namespace-id` `string`

	ID of the security namespace to query (required).

* `--recurse`

	Include child ACLs for the specified token when supported.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--token` `string`

	Security token to filter the results.


### ALIASES

- `ls`
- `l`

### JSON Fields

`allow`, `deny`, `descriptor`, `effectiveAllow`, `effectiveDeny`, `inheritPermissions`, `inheritedAllow`, `inheritedDeny`, `token`

### Examples

```bash
# List all ACLs for a namespace using the default organization
azdo security permission list --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

# List all ACLs for a namespace in an explicit organization
azdo security permission list fabrikam --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

# List all tokens for a specific user
azdo security permission list fabrikam/contoso@example.com --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866

# List ACLs for a project-scoped group
azdo security permission list fabrikam/ProjectAlpha/vssgp.Uy0xLTktMTIzNDU2 --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866 --recurse
```

### See also

* [azdo security permission](./azdo_security_permission.md)
