package create

import (
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type createOptions struct {
	repo         string
	parentRepo   string
	sourceBranch string
	exporter     util.Exporter
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
		Aliases: []string{
			"cr",
		},
		RunE: func(c *cobra.Command, args []string) error {
			// parse positional [ORG/]PROJECT/NAME
			opts.repo = args[0]
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.parentRepo, "parent", "", "[PROJECT/]REPO to fork from (same organization)")
	cmd.Flags().StringVar(&opts.sourceBranch, "source-branch", "", "Only fork the specified branch (defaults to all branches)")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"ID", "Name", "WebUrl", "SSHUrl"})

	return cmd
}

func runCreate(ctx util.CmdContext, opts *createOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	r, err := azdo.RepositoryFromName(opts.repo)
	if err != nil {
		return err
	}

	createOpts := &git.GitRepositoryCreateOptions{
		Name: types.ToPtr(r.Name()),
	}

	gitClient, err := ctx.ClientFactory().Git(ctx.Context(), r.Organization())
	if err != nil {
		return err
	}

	createRepoArgs := git.CreateRepositoryArgs{
		GitRepositoryToCreate: createOpts,
		Project:               types.ToPtr(r.Project()),
	}

	// handle fork parent parsing if provided
	if opts.parentRepo != "" {
		parent, err := azdo.RepositoryFromName(opts.parentRepo)
		if err != nil {
			return err
		}

		coreClient, err := ctx.ClientFactory().Core(ctx.Context(), parent.Organization())
		if err != nil {
			return err
		}

		parentProjectDetails, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
			ProjectId: types.ToPtr(parent.Project()),
		})
		if err != nil {
			return err
		}

		parentRepoDetails, err := gitClient.GetRepository(ctx.Context(), git.GetRepositoryArgs{
			RepositoryId: types.ToPtr(parent.Name()),
			Project:      types.ToPtr(parentProjectDetails.Id.String()),
		})
		if err != nil {
			return err
		}

		// The Azure DevOps API uses two different properties to create a fork:
		// 1. ParentRepository: A complex object in the request body that identifies the repository to be forked.
		//    This is the mandatory and fundamental way to specify that the new repository should be a fork.
		// 2. SourceRef: An optional query parameter in the URL (e.g., ?sourceRef=refs/heads/main) that specifies
		//    which refs (branches or tags) to include in the newly created fork. If omitted, all refs are copied.
		createOpts.ParentRepository = &git.GitRepositoryRef{
			Id: parentRepoDetails.Id,
			Project: &core.TeamProjectReference{
				Id: parentProjectDetails.Id,
			},
			Name: parentProjectDetails.Name,
		}
		if opts.sourceBranch != "" {
			sourceRef := "refs/heads/" + strings.TrimPrefix(opts.sourceBranch, "refs/heads/")
			createRepoArgs.SourceRef = &sourceRef
		}
	} else if opts.sourceBranch != "" {
		return util.FlagErrorf("--source-branch can only be used with --parent")
	}

	res, err := gitClient.CreateRepository(ctx.Context(), createRepoArgs)
	if err != nil {
		return err
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		iostreams, err := ctx.IOStreams()
		if err != nil {
			return err
		}
		jd := struct {
			ID      string
			Name    string
			Project string
			SshUrl  *string `json:"SshUrl,omitempty"`
			WebUrl  *string `json:"WebUrl,omitempty"`
		}{
			ID:      res.Id.String(),
			Name:    *res.Name,
			Project: r.Project(),
			SshUrl:  res.SshUrl,
			WebUrl:  res.WebUrl,
		}
		return opts.exporter.Write(iostreams, jd)
	}

	// Always use printer for output; it will handle table or JSON based on opts.format
	tp.AddColumns("ID", "Name", "Project", "SshUrl", "WebUrl")
	tp.AddField(res.Id.String())
	tp.AddField(*res.Name)
	tp.AddField(r.Project())
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
