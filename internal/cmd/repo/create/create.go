package create

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type createOptions struct {
	repo       string
	parentRepo string
	format     string
	exporter   util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Short: "Create a new repository in a project",
		Use:   "create [ORGANIZATION/]<PROJECT>/<NAME>",
		Example: heredoc.Doc(`
            # create a repository in specified project (org from default config)
            azdo repo create myproject/myrepo

            # create a repository in specified org/project
            azdo repo create myorg/myproject/myrepo

            # create a fork of an existing repo in another project
            azdo repo create myproject/myfork --parent otherproject/otherrepo
        `),
		Args: util.ExactArgs(1, "cannot create: project/repo name required"),
		RunE: func(c *cobra.Command, args []string) error {
			// parse positional [ORG/]PROJECT/NAME
			opts.repo = args[0]
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.parentRepo, "parent", "", "[PROJECT/]REPO to fork from (same organization)")
	util.StringEnumFlag(cmd, &opts.format, "format", "", "table", []string{"json"}, "Output format")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"Id", "Name", "WebUrl"})

	return cmd
}

func runCreate(ctx util.CmdContext, opts *createOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	n := strings.Split(opts.repo, "/")
	var organization, project, name string
	switch len(n) {
	case 2:
		project = n[0]
		name = n[1]
	case 3:
		organization = n[0]
		project = n[1]
		name = n[2]
	default:
		return util.FlagErrorf("invalid value %q, expected [ORGANIZATION/]PROJECT/NAME", opts.repo)
	}

	cfg, err := ctx.Config()
	if err != nil {
		return util.FlagErrorf("error getting configuration: %w", err)
	}

	if organization == "" {
		organization, err = cfg.Authentication().GetDefaultOrganization()
		if err != nil {
			return err
		}
		if organization == "" {
			return fmt.Errorf("no default organization")
		}
	}

	createOpts := &git.GitRepositoryCreateOptions{
		Name:    &name,
		Project: &core.TeamProjectReference{Name: &project},
	}

	// handle fork parent parsing if provided
	if opts.parentRepo != "" {
		parts := strings.Split(opts.parentRepo, "/")
		var parentProject, parentRepo string
		switch len(parts) {
		case 1:
			// Only repo name given → same project/org as target
			parentRepo = parts[0]
			parentProject = project
		case 2:
			// project/repo given → same org as target
			parentProject = parts[0]
			parentRepo = parts[1]
		case 3:
			// org/project/repo given → must match org of new repo
			parentOrg := parts[0]
			parentProject = parts[1]
			parentRepo = parts[2]
			if parentOrg != organization {
				return util.FlagErrorf("organization for new repo and parent repo must match")
			}
		default:
			return util.FlagErrorf("invalid parent value %q", opts.parentRepo)
		}
		createOpts.ParentRepository = &git.GitRepositoryRef{
			Name: &parentRepo,
			Project: &core.TeamProjectReference{
				Name: &parentProject,
			},
		}
	}

	gitClient, err := ctx.ClientFactory().Git(ctx.Context(), organization)
	if err != nil {
		return err
	}

	res, err := gitClient.CreateRepository(ctx.Context(), git.CreateRepositoryArgs{
		GitRepositoryToCreate: createOpts,
		Project:               &project,
	})
	if err != nil {
		return err
	}

	tp, err := ctx.Printer(opts.format)
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	// Always use printer for output; it will handle table or JSON based on opts.format
	tp.AddColumns("ID", "Name", "Project", "SSHUrl", "HTTPUrl")
	tp.AddField(res.Id.String())
	tp.AddField(*res.Name)
	tp.AddField(project)
	if res.SshUrl != nil {
		tp.AddField(*res.SshUrl)
	} else {
		tp.AddField("")
	}
	if res.WebUrl != nil {
		tp.AddField(*res.WebUrl)
	} else {
		tp.AddField("")
	}
	tp.EndRow()
	return tp.Render()
}
