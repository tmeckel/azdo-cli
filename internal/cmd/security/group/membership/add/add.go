package add

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
	exporter util.Exporter
}

type addResult struct {
	GroupDescriptor     string `json:"groupDescriptor"`
	GroupDisplayName    string `json:"groupDisplayName,omitempty"`
	MemberDescriptor    string `json:"memberDescriptor"`
	MemberDisplayName   string `json:"memberDisplayName,omitempty"`
	MemberSubjectKind   string `json:"memberSubjectKind,omitempty"`
	MemberOrigin        string `json:"memberOrigin,omitempty"`
	MemberOriginID      string `json:"memberOriginId,omitempty"`
	RelationshipCreated bool   `json:"relationshipCreated"`
	Status              string `json:"status,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "add [ORGANIZATION/]GROUP | [ORGANIZATION/]PROJECT/GROUP",
		Short: "Add a member to an Azure DevOps security group.",
		Long: heredoc.Doc(`
			Add a user or group as a member to an Azure DevOps security group.

			The positional argument accepts either ORGANIZATION/GROUP or ORGANIZATION/PROJECT/GROUP.
			Use --member to provide the member's email, descriptor, or principal name.
		`),
		Example: heredoc.Doc(`
			# Add a user by email to an organization-level group
			azdo security group membership add MyOrg/Project Administrators --member user@example.com

			# Add a group by descriptor to a project-level group
			azdo security group membership add MyOrg/MyProject/Readers --member vssgp.Uy0xLTItMw==
		`),
		Args: cobra.ExactArgs(1),
		Aliases: []string{
			"a",
			"create",
			"cr",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.scope = args[0]
			return runAdd(ctx, o)
		},
	}

	cmd.Flags().StringSliceVarP(&o.members, "member", "m", nil, "List of (comma-separated) Email, descriptor, or principal name of the user or group to add.")
	_ = cmd.MarkFlagRequired("member")
	util.AddJSONFlags(cmd, &o.exporter, []string{
		"groupDescriptor",
		"groupDisplayName",
		"memberDescriptor",
		"memberDisplayName",
		"memberSubjectKind",
		"memberOrigin",
		"memberOriginId",
		"relationshipCreated",
		"status",
	})

	return cmd
}

func runAdd(ctx util.CmdContext, o *opts) error {
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

	zap.L().Debug("resolving group for membership add",
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

	results := make([]addResult, 0, len(o.members))

	for _, rawMember := range o.members {
		memberInput := strings.TrimSpace(rawMember)
		if memberInput == "" {
			return util.FlagErrorf("member value must not be empty")
		}

		memberSubject, err := extensionsClient.ResolveMemberDescriptor(ctx.Context(), memberInput)
		if err != nil {
			return err
		}
		if memberSubject == nil || types.GetValue(memberSubject.Descriptor, "") == "" {
			return fmt.Errorf("failed to resolve member descriptor for %q", memberInput)
		}

		memberDescriptor := types.GetValue(memberSubject.Descriptor, "")

		zap.L().Debug("checking existing membership",
			zap.String("groupDescriptor", types.GetValue(group.Descriptor, "")),
			zap.String("memberDescriptor", memberDescriptor),
		)

		err = graphClient.CheckMembershipExistence(ctx.Context(), graph.CheckMembershipExistenceArgs{
			ContainerDescriptor: group.Descriptor,
			SubjectDescriptor:   types.ToPtr(memberDescriptor),
		})
		if err == nil {
			results = append(results, addResult{
				GroupDescriptor:     types.GetValue(group.Descriptor, ""),
				GroupDisplayName:    types.GetValue(group.DisplayName, target.GroupName),
				MemberDescriptor:    memberDescriptor,
				MemberDisplayName:   types.GetValue(memberSubject.DisplayName, ""),
				MemberSubjectKind:   types.GetValue(memberSubject.SubjectKind, ""),
				MemberOrigin:        types.GetValue(memberSubject.Origin, ""),
				MemberOriginID:      types.GetValue(memberSubject.OriginId, ""),
				RelationshipCreated: false,
				Status:              "already member",
			})
			continue
		}

		var wrapped *azuredevops.WrappedError
		if !errors.As(err, &wrapped) || wrapped == nil || wrapped.StatusCode == nil || *wrapped.StatusCode != http.StatusNotFound {
			return fmt.Errorf("failed to check existing membership for %q: %w", memberInput, err)
		}

		zap.L().Debug("adding membership",
			zap.String("groupDescriptor", types.GetValue(group.Descriptor, "")),
			zap.String("memberDescriptor", memberDescriptor),
			zap.String("member", memberInput),
		)

		membership, err := graphClient.AddMembership(ctx.Context(), graph.AddMembershipArgs{
			ContainerDescriptor: group.Descriptor,
			SubjectDescriptor:   types.ToPtr(memberDescriptor),
		})
		if err != nil {
			var addErr *azuredevops.WrappedError
			if errors.As(err, &addErr) && addErr != nil && addErr.StatusCode != nil && *addErr.StatusCode == http.StatusConflict {
				results = append(results, addResult{
					GroupDescriptor:     types.GetValue(group.Descriptor, ""),
					GroupDisplayName:    types.GetValue(group.DisplayName, target.GroupName),
					MemberDescriptor:    memberDescriptor,
					MemberDisplayName:   types.GetValue(memberSubject.DisplayName, ""),
					MemberSubjectKind:   types.GetValue(memberSubject.SubjectKind, ""),
					MemberOrigin:        types.GetValue(memberSubject.Origin, ""),
					MemberOriginID:      types.GetValue(memberSubject.OriginId, ""),
					RelationshipCreated: false,
					Status:              "already member",
				})
				continue
			}
			return fmt.Errorf("failed to add membership for %q: %w", memberInput, err)
		}

		results = append(results, addResult{
			GroupDescriptor:     types.GetValue(group.Descriptor, ""),
			GroupDisplayName:    types.GetValue(group.DisplayName, target.GroupName),
			MemberDescriptor:    types.GetValue(membership.MemberDescriptor, memberDescriptor),
			MemberDisplayName:   types.GetValue(memberSubject.DisplayName, ""),
			MemberSubjectKind:   types.GetValue(memberSubject.SubjectKind, ""),
			MemberOrigin:        types.GetValue(memberSubject.Origin, ""),
			MemberOriginID:      types.GetValue(memberSubject.OriginId, ""),
			RelationshipCreated: true,
			Status:              "added",
		})
	}

	ios.StopProgressIndicator()

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
