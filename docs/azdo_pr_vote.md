## Command `azdo pr vote`

```
azdo pr vote [<number> | <branch> | <url>] [flags]
```

Cast or reset your reviewer vote on an Azure DevOps pull request.

Without an argument, the pull request associated with the current branch is selected.


### Options


* `--vote` `string` (default `&#34;approve&#34;`)

	Vote value to set: {approve|approve-with-suggestions|reject|reset|wait-for-author}


### See also

* [azdo pr](./azdo_pr.md)
