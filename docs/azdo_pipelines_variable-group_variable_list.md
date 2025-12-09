## Command `azdo pipelines variable-group variable list`

```
azdo pipelines variable-group variable list [ORGANIZATION/]PROJECT/VARIABLEGROUP [flags]
```

List the variables in a variable group.

The command retrieves a variable group and lists its variables. Secret variables have their
values masked by default.

The VARIABLEGROUP can be specified by its ID or name.


### Options


* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### JSON Fields

`name`, `secret`, `value`

### Examples

```bash
# List variables in a group by ID within a project
azdo pipelines variable-groups variable list MyProject/123

# List variables in a group by name within a project and organization
azdo pipelines variable-groups variable list 'MyOrg/MyProject/My Variable Group'

# Export variables to JSON
azdo pipelines variable-groups variable list MyProject/123 --json
```

### See also

* [azdo pipelines variable-group variable](./azdo_pipelines_variable-group_variable.md)
