## Command `azdo pipelines variable-group variable delete`

```
azdo pipelines variable-group variable delete [ORGANIZATION/]PROJECT/VARIABLE_GROUP_ID_OR_NAME --name VARIABLE_NAME [flags]
```

Remove a variable from a variable group. The variable name lookup is case-insensitive.


### Options


* `--name` `string`

	Name of the variable to delete (case-insensitive)

* `--yes`

	Skip the confirmation prompt


### Examples

```bash
# Delete variable 'PASSWORD' from variable group 123 in the default organization
azdo pipelines variable-group variable delete MyProject/123 --name PASSWORD --yes
```

### See also

* [azdo pipelines variable-group variable](./azdo_pipelines_variable-group_variable.md)
