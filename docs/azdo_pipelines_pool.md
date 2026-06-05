## Command `azdo pipelines pool`

Manage Azure DevOps agent pools. Agent pools are logical groupings
of agents that target build, release, and other pipeline jobs.


### Available commands

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
```

### See also

* [azdo pipelines](./azdo_pipelines.md)
