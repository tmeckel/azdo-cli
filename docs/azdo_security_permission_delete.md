## Command `azdo security permission delete`

```
azdo security permission delete <TARGET> [flags]
```

Delete every explicit permission entry (allow or deny) for a user or group on a securable resource.

Accepted TARGET formats:
  - ORGANIZATION/SUBJECT           → delete permissions in an organization scope
  - ORGANIZATION/PROJECT/SUBJECT   → delete permissions scoped to a project


### Options


* `-n`, `--namespace-id` `string`

	ID of the security namespace to modify (required).

* `--token` `string`

	Security token to delete (required).

* `-y`, `--yes`

	Do not prompt for confirmation.


### ALIASES

- `d`
- `del`
- `rm`

### Examples

```bash
# Prompt before deleting permissions
azdo security permission delete fabrikam/contoso@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f'

# Delete permissions without confirmation
azdo security permission delete fabrikam/contoso@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f' --yes

# Delete project-scoped permissions
azdo security permission delete fabrikam/ProjectAlpha/vssgp.Uy0xLTktMTIzNDU2 --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token 'repoV2/{projectId}/{repoId}'
```

### See also

* [azdo security permission](./azdo_security_permission.md)
