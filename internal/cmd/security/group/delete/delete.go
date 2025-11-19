package delete

import (
	"fmt"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type deleteOpts struct {
	scope      string
	descriptor string
	yes        bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &deleteOpts{}

	cmd := &cobra.Command{
		Use:   "delete [ORGANIZATION/GROUP | ORGANIZATION/PROJECT/GROUP]",
		Short: "Delete an Azure DevOps security group",
		Args:  cobra.ExactArgs(1),
		Aliases: []string{
			"d",
			"del",
			"rm",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scope = args[0]
			return run(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.descriptor, "descriptor", "", "Descriptor of the group to delete (required if multiple groups match)")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Do not prompt for confirmation")

	return cmd
}

func run(ctx util.CmdContext, opts *deleteOpts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	target, err := util.ParseTarget(opts.scope)
	if err != nil {
		return err
	}

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), target.Organization)
	if err != nil {
		return err
	}

	zap.L().Debug("Resolving group for deletion", zap.String("organization", target.Organization), zap.String("project", target.Project), zap.String("group", target.Target))
	targetGroup, err := shared.FindGroupByName(ctx, target.Organization, target.Project, target.Target, opts.descriptor)
	if err != nil {
		return err
	}

	ios.StopProgressIndicator()

	if targetGroup == nil || targetGroup.Descriptor == nil || types.GetValue(targetGroup.Descriptor, "") == "" {
		return fmt.Errorf("target descriptor is nil")
	}

	// Confirmation prompt
	if !opts.yes {
		p, err := ctx.Prompter()
		if err != nil {
			return err
		}
		confirmed, err := p.Confirm(fmt.Sprintf("Delete security group %q?", target.Target), false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	// Perform delete
	err = graphClient.DeleteGroup(ctx.Context(), graph.DeleteGroupArgs{
		GroupDescriptor: targetGroup.Descriptor,
	})
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	fmt.Fprintf(ios.Out, "Deleted security group %q.\n", target.Target)
	return nil
}
