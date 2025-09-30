package delete

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deleteOptions struct {
	repository string
	yes        bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Short: "Delete a Git repository in a team project",
		Use:   "delete [organization/]project/repository",
		Example: heredoc.Doc(`
			# delete a repository in the default organization
			azdo repo delete myproject/myrepo

			# delete a repository using specified organization
			azdo repo delete myorg/myproject/myrepo
			`),
		Args: util.ExactArgs(1, "cannot delete: repository argument required"),
		Aliases: []string{
			"d",
		},
		RunE: func(c *cobra.Command, args []string) error {
			opts.repository = args[0]
			return runDelete(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Do not prompt for confirmation")

	return cmd
}

func runDelete(ctx util.CmdContext, opts *deleteOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	cs := iostreams.ColorScheme()

	r, err := azdo.RepositoryFromName(opts.repository)
	if err != nil {
		return err
	}

	if !opts.yes {
		if !iostreams.CanPrompt() {
			return util.FlagErrorf("--yes required when not running interactively")
		}
		p, err := ctx.Prompter()
		if err != nil {
			return err
		}
		confirmed, err := p.Confirm(fmt.Sprintf("Delete repository %s?", cs.Bold(r.FullName())), false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	gitClient, err := r.GitClient(ctx.Context(), ctx.ConnectionFactory())
	if err != nil {
		return err
	}

	repo, err := r.GitRepository(ctx.Context(), gitClient)
	if err != nil {
		return err
	}

	err = gitClient.DeleteRepository(ctx.Context(), git.DeleteRepositoryArgs{
		RepositoryId: repo.Id,
		Project:      types.ToPtr(r.Project()),
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(iostreams.Out, "%s Repository %s deleted\n", cs.SuccessIcon(), cs.Bold(r.FullName()))

	return nil
}
