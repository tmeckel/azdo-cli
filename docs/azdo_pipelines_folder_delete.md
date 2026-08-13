## Command `azdo pipelines folder delete`

```
azdo pipelines folder delete [ORG:]PROJECT/PATH [flags]
```

Delete a build definition folder at PATH under PROJECT.

Mirrors 'az pipelines folder delete'. The folder, all build definitions
in it, and all builds for those definitions are deleted. This action is
not reversible.


### Options


* `-y`, `--yes`

	Skip the confirmation prompt.


### ALIASES

- `d`
- `del`
- `rm`

### Examples

```bash
# Delete a folder in the default organization
azdo pipelines folder delete Fabrikam/External/CI --yes

# Delete a folder in a specific organization
azdo pipelines folder delete myorg:Fabrikam/External/CI
```

### See also

* [azdo pipelines folder](./azdo_pipelines_folder.md)
