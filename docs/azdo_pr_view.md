## Command `azdo pr view`

```
azdo pr view [<number> | <branch> | <url>] [flags]
```

Display the title, body, and other information about a pull request.

Without an argument, the pull request that belongs to the current branch
is displayed.
%!(EXTRA string=`)

### Options


* `--comment-sort` `string` (default `&#34;desc&#34;`)

	Sort comments by creation time; defaults to &#39;desc&#39; (newest first): {desc|asc}

* `--comment-type` `string` (default `&#34;text&#34;`)

	Filter comments by type; defaults to &#39;text&#39;: {text|system|all}

* `-c`, `--comments`

	View pull request comments

* `-C`, `--commits`

	View pull request commits

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields. Prefix a field with &#39;-&#39; to exclude it.

* `-r`, `--raw`

	View pull request raw

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### ALIASES

- `show`
- `status`

### JSON Fields

`author`, `commits`, `createdOn`, `description`, `id`, `isDraft`, `labels`, `mergeStatus`, `reviewers`, `sourceBranch`, `status`, `targetBranch`, `threads`, `title`, `url`

### See also

* [azdo pr](./azdo_pr.md)
