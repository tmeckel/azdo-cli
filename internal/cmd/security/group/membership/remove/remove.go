package remove

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/security/group/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type opts struct {
	scope    string
	member   string
	yes      bool
	exporter util.Exporter
}

type removeResult struct {
	GroupDescriptor     string `json:"groupDescriptor"`
	GroupDisplayName    string `json:"groupDisplayName,omitempty"`
	MemberDescriptor    string `json:"memberDescriptor"`
	MemberDisplayName   string `json:"memberDisplayName,omitempty"`
	MemberSubjectKind   string `json:"memberSubjectKind,omitempty"`
	RelationshipRemoved bool   `json:"relationshipRemoved"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "remove [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP",
		Short: "Remove a member from an Azure DevOps security group.",
		Long: heredoc.Doc(`
			Remove a user or group from an Azure DevOps security group.

			The positional argument accepts either ORGANIZATION/GROUP or ORGANIZATION/PROJECT/GROUP.
			Use --member to provide the member's email, descriptor, or principal name.
		`),
		Example: heredoc.Doc(`
			# Remove a user by email from an organization-level group
			azdo security group membership remove MyOrg/Project Administrators --member user@example.com
		`),
		Args: cobra.ExactArgs(1),
		Aliases: []string{
			"d",
			"r",
			"rm",
			"delete",
			"del",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.scope = args[0]
			return runRemove(ctx, o)
		},
	}

	cmd.Flags().StringVarP(&o.member, "member", "m", "", "Email, descriptor, or principal name of the user or group to remove.")
	_ = cmd.MarkFlagRequired("member")
	cmd.Flags().BoolVarP(&o.yes, "yes", "y", false, "Do not prompt for confirmation.")
	util.AddJSONFlags(cmd, &o.exporter, []string{
		"groupDescriptor",
		"groupDisplayName",
		"memberDescriptor",
		"memberDisplayName",
		"memberSubjectKind",
		"relationshipRemoved",
	})

	return cmd
}

func runRemove(ctx util.CmdContext, o *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	target, err := shared.ParseTargetWithDefault(ctx, o.scope)
	if err != nil {
		return err
	}
	organization := target.Organization
	project := target.Project

	zap.L().Debug("resolving group for membership removal",
		zap.String("organization", organization),
		zap.String("project", project),
		zap.String("group", target.GroupName),
	)

	group, err := shared.FindGroupByName(ctx, organization, project, target.GroupName, "")
	if err != nil {
		return err
	}
	if group == nil || group.Descriptor == nil || types.GetValue(group.Descriptor, "") == "" {
		return fmt.Errorf("resolved group descriptor is empty")
	}

	memberSubject, err := shared.ResolveMemberDescriptor(ctx, organization, o.member)
	if err != nil {
		return err
	}
	memberDescriptor := types.GetValue(memberSubject.Descriptor, "")
	if memberDescriptor == "" {
		return fmt.Errorf("failed to resolve member descriptor")
	}

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), organization)
	if err != nil {
		return err
	}

	zap.L().Debug("checking membership before removal",
		zap.String("groupDescriptor", types.GetValue(group.Descriptor, "")),
		zap.String("memberDescriptor", memberDescriptor),
	)

	err = graphClient.CheckMembershipExistence(ctx.Context(), graph.CheckMembershipExistenceArgs{
		ContainerDescriptor: group.Descriptor,
		SubjectDescriptor:   types.ToPtr(memberDescriptor),
	})
	if err != nil {
		var wrapped *azuredevops.WrappedError
		if errors.As(err, &wrapped) && wrapped != nil && wrapped.StatusCode != nil && *wrapped.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%s (%s) is not a member of %s", o.member, memberDescriptor, target.GroupName)
		}
		return fmt.Errorf("failed to verify existing membership: %w", err)
	}

	ios.StopProgressIndicator()

	if !o.yes {
		p, err := ctx.Prompter()
		if err != nil {
			return err
		}
		displayName := types.GetValue(memberSubject.DisplayName, memberDescriptor)
		confirmMessage := fmt.Sprintf("Remove %s (%s) from group %q?", o.member, displayName, target.GroupName)
		confirmed, err := p.Confirm(confirmMessage, false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	ios.StartProgressIndicator()

	zap.L().Debug("removing membership",
		zap.String("groupDescriptor", types.GetValue(group.Descriptor, "")),
		zap.String("memberDescriptor", memberDescriptor),
	)

	err = graphClient.RemoveMembership(ctx.Context(), graph.RemoveMembershipArgs{
		ContainerDescriptor: group.Descriptor,
		SubjectDescriptor:   types.ToPtr(memberDescriptor),
	})
	if err != nil {
		var wrapped *azuredevops.WrappedError
		if errors.As(err, &wrapped) && wrapped != nil && wrapped.StatusCode != nil && *wrapped.StatusCode == http.StatusNotFound {
			return fmt.Errorf("%s is not a member of %s", memberDescriptor, target.GroupName)
		}
		return fmt.Errorf("failed to remove membership: %w", err)
	}

	ios.StopProgressIndicator()

	result := removeResult{
		GroupDescriptor:     types.GetValue(group.Descriptor, ""),
		GroupDisplayName:    types.GetValue(group.DisplayName, target.GroupName),
		MemberDescriptor:    memberDescriptor,
		MemberDisplayName:   types.GetValue(memberSubject.DisplayName, ""),
		MemberSubjectKind:   types.GetValue(memberSubject.SubjectKind, ""),
		RelationshipRemoved: true,
	}

	if o.exporter != nil {
		return o.exporter.Write(ios, result)
	}

	display := result.MemberDescriptor
	if result.MemberDisplayName != "" {
		display = fmt.Sprintf("%s (%s)", result.MemberDisplayName, result.MemberDescriptor)
	}

	fmt.Fprintf(ios.Out, "Removed %s from group %q\n", display, target.GroupName)
	return nil
}
