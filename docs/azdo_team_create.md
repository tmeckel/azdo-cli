## Command `azdo team create`

```
azdo team create [ORGANIZATION/]PROJECT [flags]
```

Create a new team in the specified project. The --name flag is required.
The project argument is required; the organization falls back to the
configured default when omitted.


### Options


* `--description` `string`

	Description of the new team

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--name` `string`

	Name of the new team (required)

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `c`
- `cr`
- `new`
- `n`
- `add`
- `a`

### JSON Fields

`description`, `id`, `identity`, `identityUrl`, `name`, `projectId`, `projectName`, `url`

### Examples

```bash
# Create a team in the default organization
azdo team create Fabrikam --name "Fabrikam Engineering"

# Create a team with a description
azdo team create MyOrg/Fabrikam --name "My Team" --description "Owns the web app"
```

### See also

* [azdo team](./azdo_team.md)
