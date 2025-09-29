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
		Name: &name,
	}

	gitClient, err := ctx.ClientFactory().Git(ctx.Context(), organization)
	if err != nil {
		return err
	}

	createRepoArgs := git.CreateRepositoryArgs{
		GitRepositoryToCreate: createOpts,
		Project:               &project,
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
		coreClient, err := ctx.ClientFactory().Core(ctx.Context(), organization)
		if err != nil {
			return err
		}

		parentProjectDetails, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
			ProjectId: &parentProject,
		})
		if err != nil {
			return err
		}

		parentRepoDetails, err := gitClient.GetRepository(ctx.Context(), git.GetRepositoryArgs{
			RepositoryId: &parentRepo,
			Project:      &parentProject,
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
			Name: &parentRepo,
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
			Project: project,
			SshUrl:  res.SshUrl,
			WebUrl:  res.WebUrl,
		}
		return opts.exporter.Write(iostreams, jd)
	}

	// Always use printer for output; it will handle table or JSON based on opts.format
	tp.AddColumns("ID", "Name", "Project", "SshUrl", "WebUrl")
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
