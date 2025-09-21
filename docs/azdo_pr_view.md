## azdo pr view
```
azdo pr view [<number> | <branch> | <url>] [flags]
```
Display the title, body, and other information about a pull request.

Without an argument, the pull request that belongs to the current branch
is displayed.
%!(EXTRA string=`)
### Options


* `--comment-sort` `string`

	Sort comments by creation time; defaults to &#39;desc&#39; (newest first): {desc|asc}

* `--comment-type` `string`

	Filter comments by type; defaults to &#39;text&#39;: {text|system|all}

* `-c`, `--comments`

	View pull request comments

* `-C`, `--commits`

	View pull request commits

* `--format` `string`

	Output format: {json}

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `-r`, `--raw`

	View pull request raw

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### See also

* [azdo pr](./azdo_pr.md)
