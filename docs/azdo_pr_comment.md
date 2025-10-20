## Command `azdo pr comment`

```
azdo pr comment [<number> | <branch> | <url>] [flags]
```

Comment an existing pull request.

Without an argument, the pull request that belongs to the current branch is updated.
If there are more than one pull request associated with the current branch, one pull request must be selected explicitly.
%!(EXTRA string=`)

### Options


* `-c`, `--comment` `string`

	Comment to add to the pull request. Use &#39;-&#39; to read from stdin.

* `-t`, `--thread` `int`

	ID of the thread to reply to.


### See also

* [azdo pr](./azdo_pr.md)
