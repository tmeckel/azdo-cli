package delete

import (
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/spf13/cobra"
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

	// Parse scope argument
	parts := strings.Split(opts.scope, "/")
	var orgName, projectName, groupName string
	zap.L().Debug("Parsed scope parts", zap.Strings("parts", parts))
	switch len(parts) {
	case 2:
		zap.L().Debug("Detected organization/group format")
		orgName, groupName = parts[0], parts[1]
	case 3:
		zap.L().Debug("Detected organization/project/group format")
		orgName, projectName, groupName = parts[0], parts[1], parts[2]
	default:
		return fmt.Errorf("invalid scope format: must be ORGANIZATION/GROUP or ORGANIZATION/PROJECT/GROUP")
	}

	// Establish clients
	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), orgName)
	if err != nil {
		return err
	}

	// If projectName is given, resolve its descriptor (scope lookup)
	var scopeDescriptor *string = nil
	if projectName != "" {
		zap.L().Debug("Resolving project scope descriptor", zap.String("project", projectName))
		coreClient, err := ctx.ClientFactory().Core(ctx.Context(), orgName)
		if err != nil {
			return err
		}
		project, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
			ProjectId: &projectName,
		})
		if err != nil {
			return fmt.Errorf("failed to get project: %w", err)
		}

		desc, err := graphClient.GetDescriptor(ctx.Context(), graph.GetDescriptorArgs{
			StorageKey: project.Id,
		})
		if err != nil {
			return fmt.Errorf("failed to get scope descriptor: %w", err)
		}
		if desc.Value == nil {
			return fmt.Errorf("scope descriptor is nil")
		}
		scopeDescriptor = desc.Value
	}

	var matchingGroups []graph.GraphGroup
	var continuationToken *string
	for {
		// Lookup groups by name
		searchArgs := graph.ListGroupsArgs{
			ContinuationToken: continuationToken,
			ScopeDescriptor:   scopeDescriptor,
		}
		response, err := graphClient.ListGroups(ctx.Context(), searchArgs)
		if err != nil {
			return fmt.Errorf("failed to list groups: %w", err)
		}

		for _, g := range *response.GraphGroups {
			if strings.EqualFold(types.GetValue(g.DisplayName, ""), groupName) {
				matchingGroups = append(matchingGroups, g)
			}
		}
		if response.ContinuationToken == nil || len(*response.ContinuationToken) == 0 || (*response.ContinuationToken)[0] == "" {
			break
		}
	}

	var targetDescriptor *string
	switch len(matchingGroups) {
	case 0:
		return fmt.Errorf("no group found with name %q", groupName)
	case 1:
		targetDescriptor = matchingGroups[0].Descriptor
	default:
		if opts.descriptor == "" {
			return fmt.Errorf("multiple groups found with the given name; please specify --descriptor")
		}
		// Find group matching the provided descriptor
		for _, g := range matchingGroups {
			if types.GetValue(g.Descriptor, "") == opts.descriptor {
				targetDescriptor = g.Descriptor
				break
			}
		}
		if targetDescriptor == nil {
			return fmt.Errorf("no group found with the specified descriptor %q", opts.descriptor)
		}
	}

	if targetDescriptor == nil {
		return fmt.Errorf("target descriptor is nil")
	}

	// Confirmation prompt
	if !opts.yes {
		p, err := ctx.Prompter()
		if err != nil {
			return err
		}
		confirmed, err := p.Confirm(fmt.Sprintf("Delete security group %q?", groupName), false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	// Perform delete
	err = graphClient.DeleteGroup(ctx.Context(), graph.DeleteGroupArgs{
		GroupDescriptor: targetDescriptor,
	})
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	fmt.Fprintf(ios.Out, "Deleted security group %q.\n", groupName)
	return nil
}
