## Command `azdo pr edit`

```
azdo pr edit [<number> | <branch> | <url>] [flags]
```

Edit an existing pull request.

Without an argument, the pull request that belongs to the current branch is selected.
If there are more than one pull request associated with the current branch, one pull request will be selected based on the shared finder logic.

The command can:
- Add reviewers as optional or required, promoting/demoting existing reviewers when needed.
- Remove reviewers regardless of their current required/optional state.
- Add or remove labels

Examples:
  `azdo pr edit --add-required-reviewer alice@example.com bob@example.com`
  `azdo pr edit --add-optional-reviewer alice@example.com --remove-reviewer bob@example.com`
  `azdo pr edit --add-label bug --remove-label needs-review`


### Options


* `--add-label` `strings`

	Add labels (comma-separated)

* `--add-optional-reviewer` `strings`

	Add or demote optional reviewers (comma-separated)

* `--add-required-reviewer` `strings`

	Add or promote required reviewers (comma-separated)

* `-B`, `--base` `string`

	Change the base branch for this pull request

* `-b`, `--body` `string`

	Set the new body.

* `-F`, `--body-file` `string`

	Read body text from file (use &#34;-&#34; to read from standard input)

* `--remove-label` `strings`

	Remove labels (comma-separated, use * to remove all)

* `--remove-reviewer` `strings`

	Remove reviewers (comma-separated, use * to remove all)

* `-t`, `--title` `string`

	Set the new title.


### See also

* [azdo pr](./azdo_pr.md)
