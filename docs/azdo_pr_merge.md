## Command `azdo pr merge`

```
azdo pr merge <number> | <branch> | <url> [flags]
```

Merge a pull request on Azure DevOps.

Without an argument, the pull request that belongs to the current branch
is selected.

If required checks have not yet passed, auto-complete will be enabled.
%!(EXTRA string=`)

### Options


* `-d`, `--delete-source-branch`

	Delete the source branch after merging

* `--merge-strategy` `string`

	Merge strategy to use: {noFastForward|squash|rebase|rebaseMerge}

* `-m`, `--message` `string`

	Message to include when completing the pull request

* `--transition-work-items`

	Transition linked work item statuses upon merging


### See also

* [azdo pr](./azdo_pr.md)
