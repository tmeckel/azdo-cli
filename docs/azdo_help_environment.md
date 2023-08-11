## azdo environment
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

### See also

* [azdo](./azdo.md)
