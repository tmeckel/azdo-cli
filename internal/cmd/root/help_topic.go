package root

import (
	"fmt"
	"io"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/text"
)

type helpTopic struct {
	name    string
	short   string
	long    string
	example string
}

var HelpTopics = []helpTopic{
	{
		name:  "mintty",
		short: "Information about using azdo with MinTTY",
		long: heredoc.Doc(`
			MinTTY is the terminal emulator that comes by default with Git
			for Windows. It has known issues with azdo's ability to prompt a
			user for input.

			There are a few workarounds to make azdo work with MinTTY:

			- Reinstall Git for Windows, checking "Enable experimental support for pseudo consoles".

			- Use a different terminal emulator with Git for Windows like Windows Terminal.
			  You can run "C:\Program Files\Git\bin\bash.exe" from any terminal emulator to continue
			  using all of the tooling in Git For Windows without MinTTY.

			- Prefix invocations of azdo with winpty, eg: "winpty azdo auth login".
			  NOTE: this can lead to some UI bugs.
		`),
	},
	{
		name:  "environment",
		short: "Environment variables that can be used with azdo",
		long: heredoc.Doc(`
			AZDO_TOKEN: an authentication token for an Azure DevOps Organization
			API requests. Setting this avoids being prompted to authenticate and takes precedence over
			previously stored credentials.

			AZDO_ORGANIZATION: specify the Azure DevOps Organization URL when not in a context of an existing repository.
			When setting this, also set AZDO_TOKEN.

			AZDO_EDITOR, GIT_EDITOR, VISUAL, EDITOR (in order of precedence): the editor tool to use
			for authoring text.

			AZDO_BROWSER, BROWSER (in order of precedence): the web browser to use for opening links.

			AZDO_DEBUG: set to a truthy value to enable verbose output on standard error. Set to "api"
			to additionally log details of HTTP traffic.

			AZDO_PAGER, PAGER (in order of precedence): a terminal paging program to send standard output
			to, e.g. "less".

			GLAMOUR_STYLE: the style to use for rendering Markdown. See
			<https://github.com/charmbracelet/glamour#styles>

			NO_COLOR: set to any value to avoid printing ANSI escape sequences for color output.

			CLICOLOR: set to "0" to disable printing ANSI colors in output.

			CLICOLOR_FORCE: set to a value other than "0" to keep ANSI colors in output
			even when the output is piped.

			AZDO_FORCE_TTY: set to any value to force terminal-style output even when the output is
			redirected. When the value is a number, it is interpreted as the number of columns
			available in the viewport. When the value is a percentage, it will be applied against
			the number of columns available in the current viewport.

			AZDO_CONFIG_DIR: the directory where azdo will store configuration files. If not specified,
			the default value will be one of the following paths (in order of precedence):
			  - "$XDG_CONFIG_HOME/azdo" (if $XDG_CONFIG_HOME is set),
			  - "$AppData/AzDO CLI" (on Windows if $AppData is set), or
			  - "$HOME/.config/azdo".

			AZDO_PROMPT_DISABLED: set to any value to disable interactive prompting in the terminal.
		`),
	},
	{
		name:  "reference",
		short: "A comprehensive reference of all azdo commands",
	},
	{
		name:  "exit-codes",
		short: "Exit codes used by azdo",
		long: heredoc.Doc(`
			azdo follows normal conventions regarding exit codes.

			- If a command completes successfully, the exit code will be 0

			- If a command fails for any reason, the exit code will be 1

			- If a command is running but gets cancelled, the exit code will be 2

			- If a command encounters an authentication issue, the exit code will be 4

			NOTE: It is possible that a particular command may have more exit codes, so it is a good
			practice to check documentation for the command if you are relying on exit codes to
			control some behavior.
		`),
	},
	{
		name:  "formatting",
		short: "Formatting options for JSON data exported from gh",
		long: heredoc.Docf(`
			By default, the result of %[1]sgh%[1]s commands are output in line-based plain text format.
			Some commands support passing the %[1]s--json%[1]s flag, which converts the output to JSON format.
			Once in JSON, the output can be further formatted according to a required formatting string by
			adding either the %[1]s--jq%[1]s or %[1]s--template%[1]s flag. This is useful for selecting a subset of data,
			creating new data structures, displaying the data in a different format, or as input to another
			command line script.

			The %[1]s--json%[1]s flag requires a comma separated list of fields to fetch. To view the possible JSON
			field names for a command omit the string argument to the %[1]s--json%[1]s flag when you run the command.
			Note that you must pass the %[1]s--json%[1]s flag and field names to use the %[1]s--jq%[1]s or %[1]s--template%[1]s flags.

			The %[1]s--jq%[1]s flag requires a string argument in jq query syntax, and will only print
			those JSON values which match the query. jq queries can be used to select elements from an
			array, fields from an object, create a new array, and more. The %[1]sjq%[1]s utility does not need
			to be installed on the system to use this formatting directive. When connected to a terminal,
			the output is automatically pretty-printed. To learn about jq query syntax, see:
			<https://jqlang.github.io/jq/manual/>

			The %[1]s--template%[1]s flag requires a string argument in Go template syntax, and will only print
			those JSON values which match the query.
			In addition to the Go template functions in the standard library, the following functions can be used
			with this formatting directive:
			- %[1]sautocolor%[1]s: like %[1]scolor%[1]s, but only emits color to terminals
			- %[1]scolor <style> <input>%[1]s: colorize input using <https://github.com/mgutz/ansi>
			- %[1]sjoin <sep> <list>%[1]s: joins values in the list using a separator
			- %[1]spluck <field> <list>%[1]s: collects values of a field from all items in the input
			- %[1]stablerow <fields>...%[1]s: aligns fields in output vertically as a table
			- %[1]stablerender%[1]s: renders fields added by tablerow in place
			- %[1]stimeago <time>%[1]s: renders a timestamp as relative to now
			- %[1]stimefmt <format> <time>%[1]s: formats a timestamp using Go's %[1]sTime.Format%[1]s function
			- %[1]struncate <length> <input>%[1]s: ensures input fits within length
			- %[1]shyperlink <url> <text>%[1]s: renders a terminal hyperlink

			To learn more about Go templates, see: <https://golang.org/pkg/text/template/>.
		`, "`"),
		example: heredoc.Doc(`
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
			$ azdo pr list --json author --jq '.[].author.login'
			monalisa
			codercat
			cli-maintainer

			# --jq can be used to implement more complex filtering and output changes:
			$ azdo issue list --json number,title,labels --jq \
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
			azdo issue list --json title,url --template '{{range .}}{{hyperlink .url .title}}{{"\n"}}{{end}}'


			# adding the --template flag and modifying the display format
			$ azdo pr list --json number,title,headRefName,updatedAt --template \
				'{{range .}}{{tablerow (printf "#%v" .number | autocolor "green") .title .headRefName (timeago .updatedAt)}}{{end}}'

			#123  A helpful contribution      contribution-branch       about 1 day ago
			#124  Improve the docs            docs-branch               about 2 days ago
			#125  An exciting new feature     feature-branch            about 2 days ago


			# a more complex example with the --template flag which formats a pull request using multiple tables with headers:
			$ azdo pr view 3519 --json number,title,body,reviews,assignees --template \
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
		`),
	},
}

func NewCmdHelpTopic(ios *iostreams.IOStreams, ht helpTopic) *cobra.Command {
	cmd := &cobra.Command{
		Use:     ht.name,
		Short:   ht.short,
		Long:    ht.long,
		Example: ht.example,
		Hidden:  true,
		Annotations: map[string]string{
			"markdown:generate": "true",
			"markdown:basename": "azdo_help_" + ht.name,
		},
	}

	cmd.SetUsageFunc(func(c *cobra.Command) error {
		return helpTopicUsageFunc(ios.ErrOut, c)
	})

	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		helpTopicHelpFunc(ios.Out, c)
	})

	return cmd
}

func helpTopicHelpFunc(w io.Writer, command *cobra.Command) {
	fmt.Fprint(w, command.Long)
	if command.Example != "" {
		fmt.Fprintf(w, "\n\nEXAMPLES\n")
		fmt.Fprint(w, text.Indent(command.Example, "  "))
	}
}

func helpTopicUsageFunc(w io.Writer, command *cobra.Command) error {
	fmt.Fprintf(w, "Usage: azdo help %s", command.Use)
	return nil
}
