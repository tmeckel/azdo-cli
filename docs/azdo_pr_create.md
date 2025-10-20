## Command `azdo pr create`

```
azdo pr create [flags]
```

Create a pull request on Azure DevOps.

When the current branch isn't fully pushed to a git remote, a prompt will ask where
to push the branch and offer an option to fork the base repository. Use `--head` to
explicitly skip any forking or pushing behavior.

A prompt will also ask for the title and the body of the pull request. Use `--title` and
`--body` to skip this, or use `--fill` to autofill these values from git commits.
It's important to notice that if the `--title` and/or `--body` are also provided
alongside `--fill`, the values specified by `--title` and/or `--body` will
take precedence and overwrite any autofilled content.

Link an issue to the pull request by referencing the issue in the body of the pull
request. If the body text mentions `Fixes #123` or `Closes #123`, the referenced issue
will automatically get closed when the pull request gets merged.

By default, users with write access to the base repository can push new commits to the
head branch of the pull request. Disable this with `--no-maintainer-edit`.

Adding a pull request to projects requires authorization with the `project` scope.
To authorize, run `gh auth refresh -s project`.


### Options


* `-B`, `--base` `branch`

	The branch into which you want your code merged

* `-D`, `--description` `string`

	Description for the pull request

* `-F`, `--description-file` `file`

	Read description text from file (use &#34;-&#34; to read from standard input)

* `-d`, `--draft`

	Mark pull request as a draft

* `--dry-run`

	Print details instead of creating the PR. May still push git changes.

* `-f`, `--fill`

	Use commit info for title and body

* `--fill-first`

	Use first commit info for title and body

* `--fill-verbose`

	Use commits msg&#43;body for description

* `-H`, `--head` `branch`

	The branch that contains commits for your pull request (default [current branch])

* `-o`, `--optional-reviewer` `strings`

	Optional reviewers (comma-separated)

* `--recover` `string`

	Recover input from a failed run of create

* `-r`, `--required-reviewer` `strings`

	Required reviewers (comma-separated)

* `-t`, `--title` `string`

	Title for the pull request

* `--use-template`

	Use a pull request template for the description of the new pull request. The command will fail if no template is found


### ALIASES

- `new`

### Examples

```bash
$ azdo pr create --title "The bug is fixed" --description "Everything works again"
$ azdo pr create --reviewer monalisa,hubot  --reviewer myorg/team-name
$ azdo pr create --base develop --head monalisa:feature
$ azdo pr create --use-template
```

### See also

* [azdo pr](./azdo_pr.md)
