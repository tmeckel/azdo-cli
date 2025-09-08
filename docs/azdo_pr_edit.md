## azdo pr edit
```
azdo pr edit [<number> | <branch> | <url>] [flags]
```
Edit an existing pull request.

Without an argument, the pull request that belongs to the current branch is selected.
If there are more than one pull request associated with the current branch, one pull request will be selected based on the shared finder logic.
%!(EXTRA string=`)
### Options


* `--add-label` `strings`

	Add labels (comma-separated)

* `--add-optional-reviewer` `strings`

	Add optional reviewers (comma-separated)

* `--add-required-reviewer` `strings`

	Add required reviewers (comma-separated)

* `-B`, `--base` `string`

	Change the base branch for this pull request

* `-b`, `--body` `string`

	Set the new body.

* `-F`, `--body-file` `string`

	Read body text from file (use &#34;-&#34; to read from standard input)

* `--remove-label` `strings`

	Remove labels (comma-separated)

* `--remove-optional-reviewer` `strings`

	Remove optional reviewers (comma-separated)

* `--remove-required-reviewer` `strings`

	Remove required reviewers (comma-separated)

* `-t`, `--title` `string`

	Set the new title.


### See also

* [azdo pr](./azdo_pr.md)
