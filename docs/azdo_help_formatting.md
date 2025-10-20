## Command `azdo formatting`

By default, the result of `gh` commands are output in line-based plain text format.
Some commands support passing the `--json` flag, which converts the output to JSON format.
Once in JSON, the output can be further formatted according to a required formatting string by
adding either the `--jq` or `--template` flag. This is useful for selecting a subset of data,
creating new data structures, displaying the data in a different format, or as input to another
command line script.

The `--json` flag requires a comma separated list of fields to fetch. To view the possible JSON
field names for a command omit the string argument to the `--json` flag when you run the command.
Note that you must pass the `--json` flag and field names to use the `--jq` or `--template` flags.

The `--jq` flag requires a string argument in jq query syntax, and will only print
those JSON values which match the query. jq queries can be used to select elements from an
array, fields from an object, create a new array, and more. The `jq` utility does not need
to be installed on the system to use this formatting directive. When connected to a terminal,
the output is automatically pretty-printed. To learn about jq query syntax, see:
<https://jqlang.github.io/jq/manual/>

The `--template` flag requires a string argument in Go template syntax, and will only print
those JSON values which match the query.
In addition to the Go template functions in the standard library, the following functions can be used
with this formatting directive:
- `autocolor`: like `color`, but only emits color to terminals
- `color <style> <input>`: colorize input using <https://github.com/mgutz/ansi>
- `join <sep> <list>`: joins values in the list using a separator
- `pluck <field> <list>`: collects values of a field from all items in the input
- `tablerow <fields>...`: aligns fields in output vertically as a table
- `tablerender`: renders fields added by tablerow in place
- `timeago <time>`: renders a timestamp as relative to now
- `timefmt <format> <time>`: formats a timestamp using Go's `Time.Format` function
- `truncate <length> <input>`: ensures input fits within length
- `hyperlink <url> <text>`: renders a terminal hyperlink

To learn more about Go templates, see: <https://golang.org/pkg/text/template/>.


### Examples

```bash
# default output format
$ azdo pr list
Showing 23 of 23 open pull requests in cli/cli

#123  A helpful contribution          contribution-branch              about 1 day ago
#124  Improve the docs                docs-branch                      about 2 days ago
#125  An exciting new feature         feature-branch                   about 2 days ago


# adding the --json flag with a list of field names
$ azdo pr list --json number,title,author
[
  {
    "author": {
      "login": "monalisa"
    },
    "number": 123,
    "title": "A helpful contribution"
  },
  {
    "author": {
      "login": "codercat"
    },
    "number": 124,
    "title": "Improve the docs"
  },
  {
    "author": {
      "login": "cli-maintainer"
    },
    "number": 125,
    "title": "An exciting new feature"
  }
]


# adding the --jq flag and selecting fields from the array
$ gh pr list --json author --jq '.[].author.login'
monalisa
codercat
cli-maintainer

# --jq can be used to implement more complex filtering and output changes:
$ gh issue list --json number,title,labels --jq \
  'map(select((.labels | length) > 0))    # must have labels
  | map(.labels = (.labels | map(.name))) # show only the label names
  | .[:3]                                 # select the first 3 results'
  [
    {
      "labels": [
        "enhancement",
        "needs triage"
      ],
      "number": 123,
      "title": "A helpful contribution"
    },
    {
      "labels": [
        "help wanted",
        "docs",
        "good first issue"
      ],
      "number": 125,
      "title": "Improve the docs"
    },
    {
      "labels": [
        "enhancement",
      ],
      "number": 7221,
      "title": "An exciting new feature"
    }
  ]
# using the --template flag with the hyperlink helper
gh issue list --json title,url --template '{{range .}}{{hyperlink .url .title}}{{"\n"}}{{end}}'


# adding the --template flag and modifying the display format
$ gh pr list --json number,title,headRefName,updatedAt --template \
	'{{range .}}{{tablerow (printf "#%v" .number | autocolor "green") .title .headRefName (timeago .updatedAt)}}{{end}}'

#123  A helpful contribution      contribution-branch       about 1 day ago
#124  Improve the docs            docs-branch               about 2 days ago
#125  An exciting new feature     feature-branch            about 2 days ago


# a more complex example with the --template flag which formats a pull request using multiple tables with headers:
$ gh pr view 3519 --json number,title,body,reviews,assignees --template \
'{{printf "#%v" .number}} {{.title}}

{{.body}}

{{tablerow "ASSIGNEE" "NAME"}}{{range .assignees}}{{tablerow .login .name}}{{end}}{{tablerender}}
{{tablerow "REVIEWER" "STATE" "COMMENT"}}{{range .reviews}}{{tablerow .author.login .state .body}}{{end}}
'

#3519 Add table and helper template functions

Resolves #3488

ASSIGNEE  NAME
mislav    Mislav Marohnić


REVIEWER  STATE              COMMENT
mislav    COMMENTED          This is going along great! Thanks for working on this ❤️
```

### See also

* [azdo](./azdo.md)
