package remove

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

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
	members  []string
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
	Status              string `json:"status,omitempty"`
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

	cmd.Flags().StringSliceVarP(&o.members, "member", "m", nil, "List of (comma-separated) Email, descriptor, or principal name of the user or group to remove.")
	_ = cmd.MarkFlagRequired("member")
	cmd.Flags().BoolVarP(&o.yes, "yes", "y", false, "Do not prompt for confirmation.")
	util.AddJSONFlags(cmd, &o.exporter, []string{
		"groupDescriptor",
		"groupDisplayName",
		"memberDescriptor",
		"memberDisplayName",
		"memberSubjectKind",
		"relationshipRemoved",
		"status",
	})

	return cmd
}

func runRemove(ctx util.CmdContext, o *opts) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	if len(o.members) == 0 {
		return util.FlagErrorf("at least one --member value must be provided")
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

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), organization)
	if err != nil {
		return err
	}

	extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), organization)
	if err != nil {
		return err
	}

	type removalCandidate struct {
		input       string
		subject     *graph.GraphSubject
		descriptor  string
		displayName string
		exists      bool
		removed     bool
	}

	candidates := make([]removalCandidate, 0, len(o.members))

	for _, rawMember := range o.members {
		memberInput := strings.TrimSpace(rawMember)
		if memberInput == "" {
			return util.FlagErrorf("member value must not be empty")
		}

		memberSubject, err := extensionsClient.ResolveSubject(ctx.Context(), memberInput)
		if err != nil {
			return err
		}
		if memberSubject == nil || types.GetValue(memberSubject.Descriptor, "") == "" {
			return fmt.Errorf("failed to resolve member descriptor for %q", memberInput)
		}

		memberDescriptor := types.GetValue(memberSubject.Descriptor, "")
		displayName := types.GetValue(memberSubject.DisplayName, memberDescriptor)

		zap.L().Debug("checking membership before removal",
			zap.String("groupDescriptor", types.GetValue(group.Descriptor, "")),
			zap.String("memberDescriptor", memberDescriptor),
		)

		exists := true
		err = graphClient.CheckMembershipExistence(ctx.Context(), graph.CheckMembershipExistenceArgs{
			ContainerDescriptor: group.Descriptor,
			SubjectDescriptor:   types.ToPtr(memberDescriptor),
		})
		if err != nil {
			var wrapped *azuredevops.WrappedError
			if errors.As(err, &wrapped) && wrapped != nil && wrapped.StatusCode != nil && *wrapped.StatusCode == http.StatusNotFound {
				exists = false
			} else {
				return fmt.Errorf("failed to verify existing membership for %q: %w", memberInput, err)
			}
		}

		candidates = append(candidates, removalCandidate{
			input:       memberInput,
			subject:     memberSubject,
			descriptor:  memberDescriptor,
			displayName: displayName,
			exists:      exists,
		})
	}

	ios.StopProgressIndicator()

	removable := 0
	for _, c := range candidates {
		if c.exists {
			removable++
		}
	}

	if removable > 0 && !o.yes {
		p, err := ctx.Prompter()
		if err != nil {
			return err
		}

		var prompt string
		if removable == 1 {
			var name string
			for _, c := range candidates {
				if c.exists {
					name = c.displayName
					if name == "" {
						name = c.descriptor
					}
					break
				}
			}
			prompt = fmt.Sprintf("Remove %q from group %q?", name, target.GroupName)
		} else {
			prompt = fmt.Sprintf("Remove %d members from group %q?", removable, target.GroupName)
		}

		confirmed, err := p.Confirm(prompt, false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	if removable > 0 {
		ios.StartProgressIndicator()
		for i := range candidates {
			if !candidates[i].exists {
				continue
			}

			zap.L().Debug("removing membership",
				zap.String("groupDescriptor", types.GetValue(group.Descriptor, "")),
				zap.String("memberDescriptor", candidates[i].descriptor),
			)

			err = graphClient.RemoveMembership(ctx.Context(), graph.RemoveMembershipArgs{
				ContainerDescriptor: group.Descriptor,
				SubjectDescriptor:   types.ToPtr(candidates[i].descriptor),
			})
			if err != nil {
				var wrapped *azuredevops.WrappedError
				if errors.As(err, &wrapped) && wrapped != nil && wrapped.StatusCode != nil && *wrapped.StatusCode == http.StatusNotFound {
					candidates[i].exists = false
					continue
				}
				ios.StopProgressIndicator()
				return fmt.Errorf("failed to remove membership for %q: %w", candidates[i].input, err)
			}

			candidates[i].removed = true
		}
		ios.StopProgressIndicator()
	}

	results := make([]removeResult, 0, len(candidates))
	for _, c := range candidates {
		status := "not a member"
		if c.removed {
			status = "removed"
		} else if c.exists {
			status = "not removed"
		}

		results = append(results, removeResult{
			GroupDescriptor:     types.GetValue(group.Descriptor, ""),
			GroupDisplayName:    types.GetValue(group.DisplayName, target.GroupName),
			MemberDescriptor:    c.descriptor,
			MemberDisplayName:   c.displayName,
			MemberSubjectKind:   types.GetValue(c.subject.SubjectKind, ""),
			RelationshipRemoved: c.removed,
			Status:              status,
		})
	}

	if o.exporter != nil {
		return o.exporter.Write(ios, results)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}

	tp.AddColumns("Group", "Member", "Descriptor", "Status")
	tp.EndRow()
	for _, r := range results {
		tp.AddField(r.GroupDisplayName)
		if r.MemberDisplayName != "" {
			tp.AddField(r.MemberDisplayName)
		} else {
			tp.AddField(r.MemberDescriptor)
		}
		tp.AddField(r.MemberDescriptor)
		tp.AddField(r.Status)
		tp.EndRow()
	}

	return tp.Render()
}
