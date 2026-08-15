## Command `azdo boards work-item create`

```
azdo boards work-item create [ORG:]PROJECT [flags]
```

Create a work item of a given type in a project within an Azure DevOps
organization. The work item is built from a JSON Patch document assembled
from the supplied flags.


### Options


* `--area` `string`

	Area path of the new work item.

* `--assigned-to` `string`

	Identity the work item is assigned to.

* `--bypass-rules`

	Do not enforce the work item type rules on this create.

* `--description` `string`

	Description content (format set by --description-format; default markdown). Lower priority than --description-file and --description-editor.

* `--description-editor`

	Edit description in $VISUAL/$EDITOR. Highest priority description source.

* `--description-file` `stringArray`

	Read description from file (repeatable; &#34;-&#34; reads from stdin). Higher priority than --description.

* `--description-format` `string` (default `&#34;markdown&#34;`)

	Format of the description input: &#34;markdown&#34; (default) or &#34;html&#34;.

* `--discussion` `string`

	Comment to add to the new work item&#39;s discussion.

* `--expand` `string`

	Expand parameters: None, Relations, Fields, Links, All.

* `--fields` `stringArray`

	Set a field by reference name (repeatable; Ref.Name=value).

* `--iteration` `string`

	Iteration path of the new work item.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--link` `stringArray`

	Add a link to the new work item (repeatable; rel,url).

* `--open`

	Open the created work item in the default browser.

* `--parent` `int` (default `0`)

	ID of the parent work item; adds a System.LinkTypes.Hierarchy-Reverse link.

* `--priority` `int` (default `0`)

	Priority of the new work item (numeric value defined by the work item process).

* `--reason` `string`

	Reason for the state of the new work item.

* `--severity` `string`

	Severity of the new work item (value defined by the work item process, e.g. 1 - Critical).

* `--state` `string`

	Initial state of the new work item.

* `--suppress-notifications`

	Do not fire any notifications for this create.

* `--tag` `stringArray`

	Tag to add (repeatable); values are joined with &#39;; &#39;.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--title` `string`

	Title of the new work item.

* `--type` `string`

	Work item type to create (e.g. Bug, Task, User Story).

* `--validate-only`

	Only validate the create without saving the work item.


### ALIASES

- `c`
- `cr`

### JSON Fields

`_links`, `commentVersionRef`, `fields`, `id`, `relations`, `rev`, `url`

### Examples

```bash
# create a bug in the default org's Fabrikam project
azdo boards work-item create Fabrikam --type Bug --title "Login is broken"

# create from a description file
azdo boards work-item create Fabrikam --type Bug --title "Login broken" --description-file ./repro.md

# open the description in the user's $EDITOR
azdo boards work-item create Fabrikam --type Bug --title "Login broken" --description-editor
```

### See also

* [azdo boards work-item](./azdo_boards_work-item.md)
