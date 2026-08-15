## Command `azdo boards work-item update`

```
azdo boards work-item update [ORG:]PROJECT/ID [flags]
```

Update one or more fields of an existing work item. The work item is
identified by ID. Build a JSON Patch document from the supplied flags
and send it to the server. At least one field flag is required.


### Options


* `--area` `string`

	New area path of the work item.

* `--assigned-to` `string`

	Identity the work item is assigned to.

* `--bypass-rules`

	Do not enforce the work item type rules on this update.

* `--description` `string`

	New description content (format set by --description-format; default markdown). Lower priority than --description-file and --description-editor.

* `--description-editor`

	Edit description in $VISUAL/$EDITOR. Highest priority description source.

* `--description-file` `stringArray`

	Read description from file (repeatable; &#34;-&#34; reads from stdin). Higher priority than --description.

* `--description-format` `string` (default `&#34;markdown&#34;`)

	Format of the description input: &#34;markdown&#34; (default) or &#34;html&#34;.

* `--discussion` `string`

	Comment to add to the work item discussion.

* `--expand` `string`

	Expand parameters: None, Relations, Fields, Links, All.

* `--fields` `stringArray`

	Set a field by reference name (repeatable; Ref.Name=value).

* `--iteration` `string`

	New iteration path of the work item.

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `--open`

	Open the updated work item in the default browser.

* `--reason` `string`

	Reason for the change of state.

* `--state` `string`

	New state of the work item.

* `--suppress-notifications`

	Do not fire any notifications for this change.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `--title` `string`

	New title of the work item.

* `--validate-only`

	Only validate the changes without saving the work item.


### ALIASES

- `u`

### JSON Fields

`_links`, `commentVersionRef`, `fields`, `id`, `relations`, `rev`, `url`

### Examples

```bash
# update a work item's title
azdo boards work-item update Fabrikam/1234 --title "New title"

# update description from a Markdown file
azdo boards work-item update Fabrikam/1234 --description-file ./updated-repro.md

# edit description in $EDITOR
azdo boards work-item update Fabrikam/1234 --description-editor
```

### See also

* [azdo boards work-item](./azdo_boards_work-item.md)
