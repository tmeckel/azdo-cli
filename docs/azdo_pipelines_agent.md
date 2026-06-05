## Command `azdo pipelines agent`

Manage Azure DevOps pipeline agents. Agents are the compute targets
that run build, release, and other pipeline jobs. Each agent belongs
to an agent pool, which is identified by name or numeric ID.

Targets are specified in POOL/AGENT format where each component can
be a numeric ID or a name. An optional organization prefix can be
included: [ORGANIZATION/]POOL/AGENT.


### Available commands

* [azdo pipelines agent show](./azdo_pipelines_agent_show.md)

### ALIASES

- `agents`
- `a`

### Examples

```bash
# Show agent by pool ID and agent ID
azdo pipelines agent show 1/42

# Show agent by pool name and agent name
azdo pipelines agent show 'Default/my-agent'

# Show agent in a different organization
azdo pipelines agent show 'myorg/1/42'

# Show agent with system and user capabilities
azdo pipelines agent show 1/42 --include-capabilities
```

### See also

* [azdo pipelines](./azdo_pipelines.md)
