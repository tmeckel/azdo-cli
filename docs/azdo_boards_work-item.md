## Command `azdo boards work-item`

Work with Azure Boards work items.

### Available commands

* [azdo boards work-item create](./azdo_boards_work-item_create.md)
* [azdo boards work-item delete](./azdo_boards_work-item_delete.md)
* [azdo boards work-item list](./azdo_boards_work-item_list.md)
* [azdo boards work-item relation](./azdo_boards_work-item_relation.md)
* [azdo boards work-item show](./azdo_boards_work-item_show.md)
* [azdo boards work-item update](./azdo_boards_work-item_update.md)

### Examples

```bash
# List work items in a project
azdo boards work-item list Fabrikam

# Create a work item
azdo boards work-item create Fabrikam --type Bug --title "Login is broken"

# Show a work item's details
azdo boards work-item show Fabrikam/42 --comments

# Update a work item's title
azdo boards work-item update Fabrikam/42 --title "New title"

# Delete a work item
azdo boards work-item delete Fabrikam/42 --yes
```

### See also

* [azdo boards](./azdo_boards.md)
