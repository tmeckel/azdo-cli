package docs

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
	"github.com/tmeckel/azdo-cli/internal/cmd/root"
	"github.com/tmeckel/azdo-cli/internal/text"
)

func printJSONFields(w io.Writer, cmd *cobra.Command) {
	raw, ok := cmd.Annotations["help:json-fields"]
	if !ok {
		return
	}

	fmt.Fprint(w, "### JSON Fields\n\n")
	fmt.Fprint(w, text.NewSliceFormatter(strings.Split(raw, ",")).WithPrepend("`").WithAppend("`").WithSort(true).String())
	fmt.Fprint(w, "\n\n")
}

func printAliases(w io.Writer, cmd *cobra.Command) {
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(w, "### ALIASES\n\n")
		for _, a := range root.BuildAliasList(cmd, cmd.Aliases) {
			fmt.Fprintf(w, "- `%s`\n", strings.TrimSpace(a))
		}
		fmt.Fprint(w, "\n")
	}
}

func printOptions(w io.Writer, cmd *cobra.Command) error {
	flags := cmd.NonInheritedFlags()
	flags.SetOutput(w)
	if flags.HasAvailableFlags() {
		fmt.Fprint(w, "### Options\n")
		if err := printFlagsMarkdown(w, flags); err != nil {
			return err
		}
		fmt.Fprint(w, "\n")
	}

	parentFlags := cmd.InheritedFlags()
	parentFlags.SetOutput(w)
	if hasNonHelpFlags(parentFlags) {
		fmt.Fprint(w, "### Options inherited from parent commands\n")
		if err := printFlagsMarkdown(w, parentFlags); err != nil {
			return err
		}
		fmt.Fprint(w, "\n")
	}
	return nil
}

func hasNonHelpFlags(fs *pflag.FlagSet) (found bool) {
	fs.VisitAll(func(f *pflag.Flag) {
		if !f.Hidden && f.Name != "help" {
			found = true
		}
	})
	return found
}

var hiddenFlagDefaults = map[string]bool{
	"false": true,
	"":      true,
	"[]":    true,
	"0s":    true,
}

var defaultValMarkdownFormats = map[string]string{
	"string":   " (default `%q`)",
	"duration": " (default `%q`)",
}

func getDefaultValueMarkdownDisplayString(f *pflag.Flag) string {
	if hiddenFlagDefaults[f.DefValue] || hiddenFlagDefaults[f.Value.Type()] {
		return ""
	}

	if dvf, found := defaultValMarkdownFormats[f.Value.Type()]; found {
		return fmt.Sprintf(dvf, f.Value)
	}
	return fmt.Sprintf(" (default `%s`)", f.Value)
}

type flagView struct {
	Name      string
	Varname   string
	Shorthand string
	DefValue  string
	Usage     string
}

var flagsMarkdownTemplate = `
{{ range .Items }}
* {{ if .Shorthand }}{{ $.BT }}-{{.Shorthand}}{{ $.BT }}, {{ end -}}
		{{ $.BT }}--{{.Name}}{{ $.BT }}{{ if .Varname }} {{ $.BT }}{{.Varname}}{{ $.BT }}{{ end }}{{.DefValue}}

	{{.Usage}}
{{ end }}
`

var mdTpl = template.Must(template.New("markdownFlags").Parse(flagsMarkdownTemplate))

func printFlagsMarkdown(w io.Writer, fs *pflag.FlagSet) error {
	var flags []flagView
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		varname, usage := pflag.UnquoteUsage(f)
		flags = append(flags, flagView{
			Name:      f.Name,
			Varname:   varname,
			Shorthand: f.Shorthand,
			DefValue:  getDefaultValueMarkdownDisplayString(f),
			Usage:     usage,
		})
	})
	data := struct {
		Items []flagView
		BT    string
	}{
		Items: flags,
		BT:    "`",
	}
	return mdTpl.Execute(w, data)
}

// genMarkdownCustom creates custom markdown output.
func genMarkdownCustom(cmd *cobra.Command, w io.Writer, linkHandler func(string) string) error {
	fmt.Fprintf(w, "## Command `%s`\n\n", cmd.CommandPath())

	hasLong := cmd.Long != ""
	if !hasLong {
		fmt.Fprintf(w, "%s\n\n", cmd.Short)
	}
	if cmd.Runnable() {
		fmt.Fprintf(w, "```\n%s\n```\n\n", cmd.UseLine())
	}
	if hasLong {
		fmt.Fprintf(w, "%s\n\n", cmd.Long)
	}

	for _, g := range root.GroupedCommands(cmd) {
		fmt.Fprintf(w, "### %s\n\n", g.Title)
		for _, subcmd := range g.Commands {
			fmt.Fprintf(w, "* [%s](%s)\n", subcmd.CommandPath(), linkHandler(cmdManualPath(subcmd)))
		}
		fmt.Fprint(w, "\n")
	}

	if err := printOptions(w, cmd); err != nil {
		return err
	}

	printAliases(w, cmd)
	printJSONFields(w, cmd)

	if len(cmd.Example) > 0 {
		fmt.Fprint(w, "### Examples\n\n```bash\n")
		fmt.Fprint(w, cmd.Example)
		fmt.Fprint(w, "```\n\n")
	}

	if cmd.HasParent() {
		p := cmd.Parent()
		fmt.Fprint(w, "### See also\n\n")
		fmt.Fprintf(w, "* [%s](%s)\n", p.CommandPath(), linkHandler(cmdManualPath(p)))
	}

	return nil
}

// GenMarkdownTreeCustom is the same as GenMarkdownTree, but
// with custom filePrepender and linkHandler.
func GenMarkdownTreeCustom(cmd *cobra.Command, dir string, filePrepender, linkHandler func(string) string) error {
	if os.Getenv("AZDO_COBRA") != "" {
		return doc.GenMarkdownTreeCustom(cmd, dir, filePrepender, linkHandler)
	}

	for _, c := range cmd.Commands() {
		_, forceGeneration := c.Annotations["markdown:generate"]
		if c.Hidden && !forceGeneration {
			continue
		}

		if err := GenMarkdownTreeCustom(c, dir, filePrepender, linkHandler); err != nil {
			return err
		}
	}

	filename := filepath.Join(dir, cmdManualPath(cmd))
	f, err := os.Create(filename) //nolint:gosec
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.WriteString(f, filePrepender(filename)); err != nil {
		return err
	}
	return genMarkdownCustom(cmd, f, linkHandler)
}

func cmdManualPath(c *cobra.Command) string {
	if basenameOverride, found := c.Annotations["markdown:basename"]; found {
		return basenameOverride + ".md"
	}
	return strings.ReplaceAll(c.CommandPath(), " ", "_") + ".md"
}
