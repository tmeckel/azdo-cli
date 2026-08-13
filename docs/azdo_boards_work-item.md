## Command `azdo boards work-item`

Work with Azure Boards work items.

### Available commands

* [azdo boards work-item delete](./azdo_boards_work-item_delete.md)
* [azdo boards work-item list](./azdo_boards_work-item_list.md)
* [azdo boards work-item update](./azdo_boards_work-item_update.md)

### Examples

```bash
# List work items in a project
azdo boards work-item list Fabrikam

# Update a work item's title
azdo boards work-item update Fabrikam/42 --title "New title"

# Delete a work item
azdo boards work-item delete Fabrikam/42 --yes
```

### See also

* [azdo boards](./azdo_boards.md)
