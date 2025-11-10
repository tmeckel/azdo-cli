## Command `azdo boards area project list`

```
azdo boards area project list [ORGANIZATION/]PROJECT [flags]
```

List Azure Boards area paths for a project. The project argument accepts the form
[ORGANIZATION/]PROJECT. When the organization segment is omitted, the default
organization from configuration is used.


### Options


* `--depth` `int` (default `1`)

	Depth of child nodes to include (use 0 to omit child nodes).

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--path` `string`

	Restrict results to a specific area path (relative paths like &#34;Payments&#34; or &#34;Payments/Sub&#34; are resolved under &lt;project&gt;/Area).

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `ls`
- `l`

### JSON Fields

`hasChildren`, `id`, `identifier`, `name`, `parentPath`, `path`

### Examples

```bash
# List the top-level area paths for Fabrikam using the default organization
azdo boards area project list Fabrikam

# List the full area tree for a project in a specific organization
azdo boards area project list myorg/Fabrikam --depth 5

# List the sub-tree under a specific area path (relative paths are resolved under <project>/Area)
azdo boards area project list myorg/Fabrikam --path Payments --depth 3
```

### See also

* [azdo boards area project](./azdo_boards_area_project.md)
