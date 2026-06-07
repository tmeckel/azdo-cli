package add

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
	exporter  util.Exporter
}

type addResultView struct {
	MemberDescriptor  *string `json:"memberDescriptor,omitempty"`
	MemberDisplayName *string `json:"memberDisplayName,omitempty"`
	MemberOrigin      *string `json:"memberOrigin,omitempty"`
	MemberOriginID    *string `json:"memberOriginId,omitempty"`
	Status            *string `json:"status,omitempty"`
}

type addView struct {
	TeamName *string         `json:"teamName,omitempty"`
	Results  []addResultView `json:"results"`
}

type resolved struct {
	input       string
	descriptor  string
	displayName string
	origin      string
	originID    string
}

func resultView(r resolved, status string) addResultView {
	return addResultView{
		MemberDescriptor:  types.ToPtr(r.descriptor),
		MemberDisplayName: types.ToPtr(r.displayName),
		MemberOrigin:      types.ToPtr(r.origin),
		MemberOriginID:    types.ToPtr(r.originID),
		Status:            types.ToPtr(status),
	}
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "add [ORGANIZATION/]PROJECT/TEAM",
		Short: "Add one or more members to a team.",
		Long: heredoc.Doc(`
			Add one or more users or groups as members of a team.

			The positional argument accepts the team's project and team name in the
			form [ORGANIZATION/]PROJECT/TEAM.
		`),
		Example: heredoc.Doc(`
			# Add a user by email
			azdo team member add Fabrikam/FabrikamEngineering/MyTeam --user user@example.com

			# Add multiple users in a single invocation
			azdo team member add Fabrikam/MyProject/MyTeam -u alice@contoso.com -u bob@contoso.com

			# Add a user by subject descriptor
			azdo team member add MyOrg/Fabrikam/MyTeam --user vssgp.Uy0xLTItMw==
		`),
		Args: util.ExactArgs(1, "team argument required"),
		Aliases: []string{
			"a",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.targetArg = args[0]
			return runAdd(ctx, o)
		},
	}

	cmd.Flags().StringSliceVarP(&o.users, "user", "u", nil, "Members to add. Accepts a descriptor, email, principal name, SID, or identity ID. Pass the flag multiple times to add several members.")
	_ = cmd.MarkFlagRequired("user")
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

func runAdd(ctx util.CmdContext, o *opts) error {
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
		"resolving team for member add",
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

	// Phase 1: resolve all members. ResolveSubjects deduplicates and trims.
	resolvedSubjects, err := extensionsClient.ResolveSubjects(ctx.Context(), o.users)
	if err != nil {
		return fmt.Errorf("failed to resolve %d member(s): %w", len(o.users), err)
	}

	seen := make(map[string]struct{})
	var unresolved []string
	resolvedMembers := make([]resolved, 0, len(o.users))
	for _, rawMember := range o.users {
		t := strings.TrimSpace(rawMember)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		subject, ok := resolvedSubjects[t]
		if !ok {
			unresolved = append(unresolved, t)
			continue
		}
		resolvedMembers = append(resolvedMembers, resolved{
			input:       t,
			descriptor:  types.GetValue(subject.Descriptor, ""),
			displayName: types.GetValue(subject.DisplayName, ""),
			origin:      types.GetValue(subject.Origin, ""),
			originID:    types.GetValue(subject.LegacyDescriptor, ""),
		})
	}
	if len(unresolved) > 0 {
		return fmt.Errorf("failed to resolve %d member(s): %s", len(unresolved), strings.Join(unresolved, ", "))
	}

	// Phase 2: check membership existence and add each resolved member.
	results := make([]addResultView, 0, len(resolvedMembers))

	for _, r := range resolvedMembers {
		zap.L().Debug(
			"checking existing membership",
			zap.String("memberDescriptor", r.descriptor),
		)

		err = graphClient.CheckMembershipExistence(ctx.Context(), graph.CheckMembershipExistenceArgs{
			ContainerDescriptor: types.ToPtr(teamGroupDescriptor),
			SubjectDescriptor:   types.ToPtr(r.descriptor),
		})
		if err == nil {
			results = append(results, resultView(r, "already member"))
			continue
		}

		var wrapped *azuredevops.WrappedError
		if !errors.As(err, &wrapped) || wrapped == nil || wrapped.StatusCode == nil || *wrapped.StatusCode != http.StatusNotFound {
			return fmt.Errorf("failed to check membership for %q: %w", r.input, err)
		}

		zap.L().Debug(
			"adding membership",
			zap.String("memberDescriptor", r.descriptor),
		)

		if _, err := graphClient.AddMembership(ctx.Context(), graph.AddMembershipArgs{
			ContainerDescriptor: types.ToPtr(teamGroupDescriptor),
			SubjectDescriptor:   types.ToPtr(r.descriptor),
		}); err != nil {
			var addErr *azuredevops.WrappedError
			if errors.As(err, &addErr) && addErr != nil && addErr.StatusCode != nil && *addErr.StatusCode == http.StatusConflict {
				results = append(results, resultView(r, "already member"))
				continue
			}
			return fmt.Errorf("failed to add member %q: %w", r.input, err)
		}

		results = append(results, resultView(r, "added"))
	}

	ios.StopProgressIndicator()

	view := addView{
		TeamName: types.ToPtr(teamName),
		Results:  results,
	}

	if o.exporter != nil {
		if err := o.exporter.Write(ios, view); err != nil {
			return err
		}
	} else {
		tp, err := ctx.Printer("list")
		if err != nil {
			return err
		}

		tp.AddColumns("MEMBER", "DESCRIPTOR", "STATUS")
		tp.EndRow()

		for _, r := range results {
			display := types.GetValue(r.MemberDisplayName, "")
			if display == "" {
				display = types.GetValue(r.MemberDescriptor, "")
			}
			tp.AddField(display)
			tp.AddField(types.GetValue(r.MemberDescriptor, ""))
			tp.AddField(types.GetValue(r.Status, ""))
			tp.EndRow()
		}

		if err := tp.Render(); err != nil {
			return err
		}
	}

	return nil
}
