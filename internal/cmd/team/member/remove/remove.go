package remove

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

type opts struct {
	targetArg string
	users     []string
	yes       bool
	exporter  util.Exporter
}

type removeResultView struct {
	MemberDescriptor  *string `json:"memberDescriptor,omitempty"`
	MemberDisplayName *string `json:"memberDisplayName,omitempty"`
	MemberOrigin      *string `json:"memberOrigin,omitempty"`
	MemberOriginID    *string `json:"memberOriginId,omitempty"`
	Status            *string `json:"status,omitempty"`
}

type removeView struct {
	TeamName *string            `json:"teamName,omitempty"`
	Results  []removeResultView `json:"results"`
}

type memberState struct {
	input       string
	descriptor  string
	displayName string
	origin      string
	originID    string
	statusVal   string
}

const (
	statusNotFound   = "not found"
	statusNotAMember = "not a member"
	statusRemoved    = "removed"
	statusError      = "error"
	statusToRemove   = ""
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "remove [ORGANIZATION/]PROJECT/TEAM",
		Short: "Remove one or more members from a team.",
		Long: heredoc.Doc(`
			Remove one or more users or groups from a team.

			The positional argument accepts the team's project and team name in the
			form [ORGANIZATION/]PROJECT/TEAM.
		`),
		Example: heredoc.Doc(`
			# Remove a user by email
			azdo team member remove Fabrikam/FabrikamEngineering/MyTeam --user user@example.com

			# Remove multiple users in a single invocation
			azdo team member remove Fabrikam/MyProject/MyTeam -u alice@contoso.com -u bob@contoso.com

			# Remove a user without confirmation prompt
			azdo team member remove MyOrg/Fabrikam/MyTeam --user vssgp.Uy0xLTItMw== --yes
		`),
		Args: util.ExactArgs(1, "team argument required"),
		Aliases: []string{
			"r",
			"rm",
			"del",
			"d",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.targetArg = args[0]
			return runRemove(ctx, o)
		},
	}

	cmd.Flags().StringSliceVarP(&o.users, "user", "u", nil, "Members to remove. Accepts a descriptor, email, principal name, SID, or identity ID. Pass the flag multiple times to remove several members.")
	_ = cmd.MarkFlagRequired("user")
	cmd.Flags().BoolVarP(&o.yes, "yes", "y", false, "Skip the confirmation prompt.")
	util.AddJSONFlags(cmd, &o.exporter, []string{
		"teamName",
		"results",
		"memberDescriptor",
		"memberDisplayName",
		"memberOrigin",
		"memberOriginId",
		"status",
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

	scope, err := util.ParseProjectTargetWithDefaultOrganization(ctx, o.targetArg)
	if err != nil {
		return err
	}

	zap.L().Debug(
		"resolving team for member remove",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("team", scope.Targets[0]),
	)

	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Core client: %w", err)
	}

	team, err := coreClient.GetTeam(ctx.Context(), core.GetTeamArgs{
		ProjectId:      types.ToPtr(scope.Project),
		TeamId:         types.ToPtr(scope.Targets[0]),
		ExpandIdentity: types.ToPtr(true),
	})
	if err != nil {
		return fmt.Errorf("failed to resolve team %q in project %q: %w", scope.Targets[0], scope.Project, err)
	}
	if team == nil || team.Identity == nil || types.GetValue(team.Identity.SubjectDescriptor, "") == "" {
		return fmt.Errorf("team has no underlying descriptor (Identity.SubjectDescriptor is empty)")
	}

	teamGroupDescriptor := types.GetValue(team.Identity.SubjectDescriptor, "")
	teamName := types.GetValue(team.Name, "")

	extensionsClient, err := ctx.ClientFactory().Extensions(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Extensions client: %w", err)
	}

	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create Graph client: %w", err)
	}

	// Deduplicate users preserving input order
	seen := make(map[string]struct{})
	uniqueUsers := make([]string, 0, len(o.users))
	for _, raw := range o.users {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		uniqueUsers = append(uniqueUsers, t)
	}

	// Phase 1: resolve all members
	members := make([]memberState, 0, len(uniqueUsers))
	for _, input := range uniqueUsers {
		subject, err := extensionsClient.ResolveSubject(ctx.Context(), input)
		if err != nil {
			members = append(members, memberState{
				input:     input,
				statusVal: statusNotFound,
			})
			continue
		}
		descriptor := types.GetValue(subject.Descriptor, "")
		displayName := types.GetValue(subject.DisplayName, descriptor)
		members = append(members, memberState{
			input:       input,
			descriptor:  descriptor,
			displayName: displayName,
			origin:      types.GetValue(subject.Origin, ""),
			originID:    types.GetValue(subject.LegacyDescriptor, ""),
			statusVal:   statusToRemove,
		})
	}

	// Phase 2: check membership for each resolved member
	for i := range members {
		if members[i].descriptor == "" {
			continue
		}

		err = graphClient.CheckMembershipExistence(ctx.Context(), graph.CheckMembershipExistenceArgs{
			ContainerDescriptor: types.ToPtr(teamGroupDescriptor),
			SubjectDescriptor:   types.ToPtr(members[i].descriptor),
		})
		if err == nil {
			members[i].statusVal = statusToRemove
			continue
		}

		var wrapped *azuredevops.WrappedError
		if errors.As(err, &wrapped) && wrapped != nil && wrapped.StatusCode != nil && *wrapped.StatusCode == http.StatusNotFound {
			members[i].statusVal = statusNotAMember
			continue
		}

		zap.L().Debug(
			"membership check failed",
			zap.String("member", members[i].input),
			zap.Error(err),
		)
		members[i].statusVal = statusError
	}

	ios.StopProgressIndicator()

	// Phase 3: confirmation prompt
	removable := 0
	for _, m := range members {
		if m.statusVal == statusToRemove {
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
			for _, m := range members {
				if m.statusVal == statusToRemove {
					name = m.displayName
					if name == "" {
						name = m.descriptor
					}
					if name == "" {
						name = m.input
					}
					break
				}
			}
			prompt = fmt.Sprintf("Are you sure you want to remove the member %s from team %s?", name, teamName)
		} else {
			var sb strings.Builder
			fmt.Fprintf(&sb, "Are you sure you want to remove %d members from team %s?", removable, teamName)
			listed := 0
			for _, m := range members {
				if m.statusVal == statusToRemove {
					if listed < 5 {
						name := m.displayName
						if name == "" {
							name = m.descriptor
						}
						if name == "" {
							name = m.input
						}
						fmt.Fprintf(&sb, "\n  - %s", name)
						listed++
					} else {
						fmt.Fprintf(&sb, "\n  ... and %d more", removable-listed)
						break
					}
				}
			}
			prompt = sb.String()
		}

		confirmed, err := p.Confirm(prompt, false)
		if err != nil {
			return err
		}
		if !confirmed {
			return util.ErrCancel
		}
	}

	// Phase 4: remove pending members
	if removable > 0 {
		ios.StartProgressIndicator()
		for i := range members {
			if members[i].statusVal != statusToRemove {
				continue
			}

			err = graphClient.RemoveMembership(ctx.Context(), graph.RemoveMembershipArgs{
				ContainerDescriptor: types.ToPtr(teamGroupDescriptor),
				SubjectDescriptor:   types.ToPtr(members[i].descriptor),
			})
			if err == nil {
				members[i].statusVal = statusRemoved
				continue
			}

			var wrapped *azuredevops.WrappedError
			if errors.As(err, &wrapped) && wrapped != nil && wrapped.StatusCode != nil && *wrapped.StatusCode == http.StatusNotFound {
				members[i].statusVal = statusNotAMember
				continue
			}

			zap.L().Debug(
				"failed to remove membership",
				zap.String("member", members[i].input),
				zap.Error(err),
			)
			members[i].statusVal = statusError
		}
		ios.StopProgressIndicator()
	}

	// Phase 5: assemble results
	// Compute exit code
	var failureCount int

	if o.exporter != nil {
		results := make([]removeResultView, 0, len(members))
		for _, m := range members {
			s := m.statusVal
			if s == "" {
				s = statusError
			}

			r := removeResultView{
				MemberDescriptor:  nil,
				MemberDisplayName: nil,
				MemberOrigin:      nil,
				MemberOriginID:    nil,
				Status:            types.ToPtr(s),
			}
			if m.descriptor != "" {
				r.MemberDescriptor = types.ToPtr(m.descriptor)
				r.MemberDisplayName = types.ToPtr(m.descriptor)
			}
			if m.displayName != "" {
				r.MemberDisplayName = types.ToPtr(m.displayName)
			}
			if m.origin != "" {
				r.MemberOrigin = types.ToPtr(m.origin)
			}
			if m.originID != "" {
				r.MemberOriginID = types.ToPtr(m.originID)
			}
			results = append(results, r)
		}

		view := removeView{
			TeamName: types.ToPtr(teamName),
			Results:  results,
		}

		if err := o.exporter.Write(ios, view); err != nil {
			return err
		}

		for _, r := range results {
			s := types.GetValue(r.Status, "")
			if s != statusRemoved && s != statusNotAMember {
				failureCount++
			}
		}
	} else {
		tp, err := ctx.Printer("list")
		if err != nil {
			return err
		}

		tp.AddColumns("MEMBER", "DESCRIPTOR", "STATUS")
		tp.EndRow()

		for _, m := range members {
			display := m.displayName
			if display == "" {
				display = m.descriptor
			}

			desc := m.descriptor
			if desc == "" {
				desc = m.input
			}

			s := m.statusVal
			if s == "" {
				s = statusError
			}

			tp.AddField(display)
			tp.AddField(desc)
			tp.AddField(s)
			tp.EndRow()

			if s != statusRemoved && s != statusNotAMember {
				failureCount++
			}
		}

		if err := tp.Render(); err != nil {
			return err
		}
	}

	if failureCount > 0 {
		return fmt.Errorf("remove completed with %d failure(s)", failureCount)
	}

	return nil
}
