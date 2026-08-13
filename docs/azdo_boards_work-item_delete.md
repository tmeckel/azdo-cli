## Command `azdo boards work-item delete`

```
azdo boards work-item delete [ORG:]PROJECT/ID [flags]
```

Delete a work item by ID. By default the work item is moved to the
Recycle Bin and can be restored via the Azure DevOps web UI.
Use --destroy to permanently remove the work item; this cannot be
undone.


### Options


* `--destroy`

	Permanently delete the work item (bypasses Recycle Bin).

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;

* `-y`, `--yes`

	Skip the confirmation prompt.


### ALIASES

- `d`
- `del`
- `rm`

### JSON Fields

`code`, `deletedBy`, `deletedDate`, `id`, `message`, `name`, `project`, `resource`, `type`, `url`

### Examples

```bash
# Delete a work item in the default organization
azdo boards work-item delete Fabrikam/42 --yes

# Permanently destroy a work item in a specific organization
azdo boards work-item delete myorg:Fabrikam/42 --destroy --yes
```

### See also

* [azdo boards work-item](./azdo_boards_work-item.md)
