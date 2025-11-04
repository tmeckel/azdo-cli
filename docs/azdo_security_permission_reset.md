## Command `azdo security permission reset`

```
azdo security permission reset <TARGET> [flags]
```

Reset the explicit allow/deny permission bits for a user or group on a securable resource (identified by a token).

The --permission-bit flag accepts one or more permission values. Each value may be provided as:
  - a hexadecimal bitmask (e.g. 0x4),
  - a decimal bit value (e.g. 4), or
  - a textual action name or display name matching the namespace action (e.g. "Read").

Accepted TARGET formats:
  - ORGANIZATION/SUBJECT
  - ORGANIZATION/PROJECT/SUBJECT


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-n`, `--namespace-id` `string`

	ID of the security namespace to modify (required).

* `--permission-bit` `strings`

	Permission bit or comma-separated bits to reset (required).

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--token` `string`

	Security token for the resource (required).

* `-y`, `--yes`

	Do not prompt for confirmation.


### ALIASES

- `r`

### JSON Fields

`bit`, `displayName`, `effective`, `name`

### Examples

```bash
# Reset the Read action (textual) for a user on a token
azdo security permission reset fabrikam/user@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f' --permission-bit Read

# Reset multiple actions by specifying --permission-bit multiple times
azdo security permission reset fabrikam/user@example.com --namespace-id bf7bfa03-b2b7-47db-8113-fa2e002cc5b1 --token vstfs:///Classification/Node/18c76992-93fa-4eb2-aac0-0abc0be212d6 --permission-bit Read --permission-bit Contribute

# Reset multiple actions using a single comma-separated value (shells may need quoting)
azdo security permission reset fabrikam/user@example.com --namespace-id 302acaca-b667-436d-a946-87133492041c --token BuildPrivileges --permission-bit "Read,Contribute,0x4"

# Use --yes to skip confirmation prompts
azdo security permission reset fabrikam/user@example.com --namespace-id 8adf73b7-389a-4276-b638-fe1653f7efc7 --token '$/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd/00000000-0000-0000-0000-000000000000' --permission-bit Read --yes
```

### See also

* [azdo security permission](./azdo_security_permission.md)
