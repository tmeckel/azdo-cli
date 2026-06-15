## Command `azdo pipelines pool`

Manage Azure DevOps agent pools. Agent pools are logical groupings
of agents that target build, release, and other pipeline jobs.


### Available commands

* [azdo pipelines pool list](./azdo_pipelines_pool_list.md)
* [azdo pipelines pool show](./azdo_pipelines_pool_show.md)

### ALIASES

- `pools`

### Examples

```bash
# Show a pool by ID
azdo pipelines pool show 42

# Show a pool by name
azdo pipelines pool show 'Default'

# Show a pool in a specific organization
azdo pipelines pool show 'myorg/Default'

# List pools in the default organization
azdo pipelines pool list

# List pools in a specific organization
azdo pipelines pool list myorg
```

### See also

* [azdo pipelines](./azdo_pipelines.md)
