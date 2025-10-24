## Command `azdo pr diff`

```
azdo pr diff [<number> | <branch> | <url>] [flags]
```

View changes in a pull request.
The output displays a list of changed files and their change types.

Without an argument, the pull request that belongs to the current branch is selected.
If there are more than one pull request associated with the current branch, one pull request will be selected based on the shared finder logic.
%!(EXTRA string=`)

### Options


* `--color` `string` (default `&#34;auto&#34;`)

	Use color in diff output: {always|never|auto}

* `--name-only`

	Display only names of changed files


### See also

* [azdo pr](./azdo_pr.md)
