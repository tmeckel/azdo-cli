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
			"markdown:basename": "gh_help_" + ht.name,
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
