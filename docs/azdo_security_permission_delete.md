## Command `azdo security permission delete`

```
azdo security permission delete [ORG:][/]SUBJECT | [ORG:]PROJECT/SUBJECT [flags]
```

Delete every explicit permission entry (allow or deny) for a user or group on a securable resource.

Accepted TARGET formats:
  - /SUBJECT                       → delete permissions in the default organization
  - ORG:/SUBJECT                   → delete permissions in an explicit organization
  - PROJECT/SUBJECT                → delete permissions scoped to the project
  - ORG:PROJECT/SUBJECT            → delete permissions scoped to the project in an explicit organization

A legacy two-segment input such as ORG/SUBJECT is interpreted as PROJECT/SUBJECT
in the default organization. Use ORG:/SUBJECT for organization-level subjects.


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
azdo security permission delete org:/contoso@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f'

# Delete permissions without confirmation
azdo security permission delete org:/contoso@example.com --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token '$/696416ee-f7ff-4ee3-934a-979b00dce74f' --yes

# Delete project-scoped permissions
azdo security permission delete org:ProjectAlpha/vssgp.Uy0xLTktMTIzNDU2 --namespace-id 71356614-aad7-4757-8f2c-0fb3bff6f680 --token 'repoV2/{projectId}/{repoId}'
```

### See also

* [azdo security permission](./azdo_security_permission.md)
