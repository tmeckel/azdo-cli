## Command `azdo auth logout`

```
azdo auth logout [ORG]
```

Remove authentication for a Azure DevOps organization.

This command removes the authentication configuration for an organization either specified
interactively or via passing an organization name.
%!(EXTRA string=`)

### Examples

```bash
$ azdo auth logout
# => select what organization to log out of via a prompt

$ azdo auth logout my-org
# => log out of specified organization
```

### See also

* [azdo auth](./azdo_auth.md)
