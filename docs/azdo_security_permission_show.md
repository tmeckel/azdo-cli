## Command `azdo security permission show`

```
azdo security permission show <TARGET> [flags]
```

Show the explicit and effective permissions for a user or group on a specific securable resource (identified by a token).

Accepted TARGET formats:
  - ORGANIZATION/SUBJECT           → show permissions for the specified subject
  - ORGANIZATION/PROJECT/SUBJECT   → show permissions for the subject scoped to the project


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-n`, `--namespace-id` `string`

	ID of the security namespace to query (required).

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--token` `string`

	Security token to query (required).


### ALIASES

- `s`

### JSON Fields

`allow`, `deny`, `descriptor`, `effectiveAllow`, `effectiveDeny`, `inheritPermissions`, `inheritedAllow`, `inheritedDeny`, `token`

### Examples

```bash
# Show permissions for a user
azdo security permission show fabrikam/contoso@example.com --namespace-id 5a27515b-ccd7-42c9-84f1-54c998f03866 --token /projects/a6880f5a-60e1-4103-89f2-69533e4d139f

# Show permissions for a project-scoped group
azdo security permission show fabrikam/ProjectAlpha/vssgp.Uy0xLTktMTIzNDU2 --namespace-id 33344d9c-fc72-4d6f-aba5-fa317101a7e9 --token /
```

### See also

* [azdo security permission](./azdo_security_permission.md)
