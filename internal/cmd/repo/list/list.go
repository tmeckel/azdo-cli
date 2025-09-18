package list

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/printer"
)

type listOptions struct {
	organizationName string
	project          string
	limit            int
	visibility       string
	includeHidden    bool
	format           string
}

func NewCmdRepoList(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Short: "List repositories of a project inside an organization",
		Use:   "list [organization/]<project>",
		Example: heredoc.Doc(`
			# list the repositories of a project using default organization
			azdo repo list myproject

			# list the repositories of a project using specified organization
			azdo repo list myorg/myproject
		`),
		Args:    util.ExactArgs(1, "cannot list: project name required"),
		Aliases: []string{"ls"},
		RunE: func(c *cobra.Command, args []string) error {
			if opts.limit < 1 {
				return util.FlagErrorf("invalid limit: %v", opts.limit)
			}

			n := strings.Split(args[0], "/")
			switch len(n) {
			case 1:
				opts.project = n[0]
			case 2:
				opts.organizationName = n[0]
				opts.project = n[1]
			default:
				return util.FlagErrorf("invalid project name %q", args[0])
			}

			return runList(ctx, opts)
		},
	}

	cmd.Flags().IntVarP(&opts.limit, "limit", "L", 30, "Maximum number of repositories to list")
	util.StringEnumFlag(cmd, &opts.visibility, "visibility", "", "", []string{"public", "private"}, "Filter by repository visibility")
	util.StringEnumFlag(cmd, &opts.format, "format", "", "table", []string{"json"}, "Output format")
	cmd.Flags().BoolVar(&opts.includeHidden, "include-hidden", false, "Include hidden repositories")

	return cmd
}

func runList(ctx util.CmdContext, opts *listOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	iostreams.StartProgressIndicator()
	defer iostreams.StopProgressIndicator()

	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting io configuration: %w", err)
	}

	var organizationName string
	if opts.organizationName != "" {
		organizationName = opts.organizationName
	} else {
		organizationName, _ = cfg.Authentication().GetDefaultOrganization()
	}
	if organizationName == "" {
		return util.FlagErrorf("no organization specified or no default organization set")
	}
	_, err = ctx.ConnectionFactory().Connection(organizationName)
	if err != nil {
		return err
	}

	repoClient, err := ctx.ConnectionFactory().Git(ctx.Context(), organizationName)
	if err != nil {
		return err
	}

	res, err := repoClient.GetRepositories(ctx.Context(), git.GetRepositoriesArgs{
		Project:       &opts.project,
		IncludeHidden: &opts.includeHidden,
	})
	if err != nil {
		return err
	}

	if res == nil || len(*res) == 0 {
		return util.NewNoResultsError(fmt.Sprintf("No repositories found for project %s and organization %s", opts.project, organizationName))
	}

	tp, err := ctx.Printer(opts.format)
	if err != nil {
		return err
	}

	sort.Slice(*res, func(i, j int) bool {
		return strings.ToLower(*((*res)[i].Name)) < strings.ToLower(*((*res)[j].Name))
	})

	iostreams.StopProgressIndicator()

	tp.AddColumns("ID", "Name", "SSHUrl", "HTTPUrl")
	for _, p := range *res {
		tp.AddField(p.Id.String(), printer.WithTruncate(nil))
		tp.AddField(*p.Name)
		tp.AddField(*p.SshUrl)
		tp.AddField(*p.WebUrl)
		tp.EndRow()
	}
	return tp.Render()
}
