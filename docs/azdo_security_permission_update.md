## Command `azdo security permission update`

```
azdo security permission update <TARGET> [flags]
```

	Update the permissions for a user or group on a specific securable resource (identified by a token) by assigning "allow" or "deny" permission bits.

	The --allow-bit and --deny-bit flags accept one or more permission values. Each value may be provided as:
	  - a hexadecimal bitmask (e.g. 0x4),
	  - a decimal bit value (e.g. 4), or
	  - a textual action name matching the namespace action (e.g. "Read", "Edit").

	To discover the available actions (and their textual names) for a security namespace, use:
	  azdo security permission namespace show --namespace-id <namespace-uuid>

	Accepted TARGET formats:
  - ORGANIZATION/SUBJECT           → target subject in org
  - ORGANIZATION/PROJECT/SUBJECT   → subject scoped to project

Token hierarchy (Git repo namespace example):
  - repoV2 → all repos across org
  - repoV2/{projectId} → all repos in project
  - repoV2/{projectId}/{repoId} → single repo in project


	  - ORGANIZATION/SUBJECT           → target subject in org
	  - ORGANIZATION/PROJECT/SUBJECT   → subject scoped to project


### Options


* `--allow-bit` `strings`

	Permission bit or comma-separated bits to allow.

* `--deny-bit` `strings`

	Permission bit or comma-separated bits to deny.

* `--merge`

	Merge incoming ACEs with existing entries or replace the permissions. If provided without value true is implied.

* `-n`, `--namespace-id` `string`

	ID of the security namespace to modify (required).

* `--token` `string`

	Security token for the resource (required).

* `-y`, `--yes`

	Do not prompt for confirmation.


### ALIASES

- `create`
- `u`
- `new`

### Examples

```bash
# Allow the Read action (textual) for a user on a token
azdo security permission update fabrikam/contoso@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f' --allow-bit Read

# Allow multiple actions by specifying --allow-bit multiple times (textual and numeric)
azdo security permission update fabrikam/contoso@example.com --namespace-id bf7bfa03-b2b7-47db-8113-fa2e002cc5b1 --token vstfs:///Classification/Node/18c76992-93fa-4eb2-aac0-0abc0be212d6 --allow-bit Read --allow-bit Contribute --allow-bit 0x4

# Allow multiple actions using a single comma-separated value (shells may need quoting)
azdo security permission update fabrikam/contoso@example.com --namespace-id 302acaca-b667-436d-a946-87133492041c --token BuildPrivileges --allow-bit "Read,Contribute,0x4"

# Deny a numeric bit and merge with existing ACEs (merge will OR incoming bits with existing ACE)
azdo security permission update fabrikam/contoso@example.com --namespace-id 33344d9c-fc72-4d6f-aba5-fa317101a7e9 --token '696416ee-f7ff-4ee3-934a-979b00dce74f/237' --deny-bit 8 --merge

# Use --yes to skip confirmation prompts
azdo security permission update fabrikam/contoso@example.com --namespace-id 8adf73b7-389a-4276-b638-fe1653f7efc7 --token '$/f6ad111f-42cb-4e2d-b22a-cd0bd6f5aebd/00000000-0000-0000-0000-000000000000' --allow-bit Read --yes
```

### See also

* [azdo security permission](./azdo_security_permission.md)
