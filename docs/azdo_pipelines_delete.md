## Command `azdo pipelines delete`

```
azdo pipelines delete [ORGANIZATION/]PROJECT/PIPELINE [flags]
```

Delete a pipeline definition by ID or name.

The command prompts for confirmation unless --yes is supplied.


### Options


* `-y`, `--yes`

	Skip the confirmation prompt.


### ALIASES

- `d`
- `del`
- `rm`

### Examples

```bash
# Delete a pipeline by ID using the default organization
azdo pipelines delete Fabrikam/42 --yes

# Delete a pipeline by name
azdo pipelines delete 'myorg/Fabrikam/My Pipeline'

# Delete with confirmation
azdo pipelines delete Fabrikam/MyPipeline
```

### See also

* [azdo pipelines](./azdo_pipelines.md)
