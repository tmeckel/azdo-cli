## azdo pr view
```
azdo pr view [<number> | <branch> | <url>] [flags]
```
Display the title, body, and other information about a pull request.

Without an argument, the pull request that belongs to the current branch
is displayed.
%!(EXTRA string=`)
### Options


* `-c`, `--comments`

	View pull request comments

* `-C`, `--commits`

	View pull request commits

* `-q`, `--jq` `expression`

	Filter JSON output using a jq expression

* `--json` `fields`

	Output JSON with the specified fields

* `-r`, `--raw`

	View pull request raw

* `-t`, `--template` `string`

	Format JSON output using a Go template; see &#34;azdo help formatting&#34;


### See also

* [azdo pr](./azdo_pr.md)
