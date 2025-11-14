package list

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type usersListOptions struct {
	organizationName string
	projectName      string
	filter           string
	subjectTypes     []string
	top              int
	exporter         util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &usersListOptions{}
	cmd := &cobra.Command{
		Use:   "list [project]",
		Short: "List users and groups in Azure DevOps",
		Long: heredoc.Doc(`
			List users and groups from an Azure DevOps project or organization.

			By default, it lists users in the organization. You can scope the search to a specific
			project by providing the project name as an argument.

			The command allows filtering by user type (e.g., 'aad', 'msa', 'svc') and supports
			prefix-based filtering on user display names.
		`),
		Example: heredoc.Doc(`
			# List all users in the default organization
			azdo graph user list

			# List all users in a specific project
			azdo graph user list "My Project"

			# List all users with the 'msa' subject type (Microsoft Account)
			azdo graph user list --type msa

			# Filter users by a name prefix
			azdo graph user list --filter "john.doe"

			# Limit the number of users returned
			azdo graph user list --limit 10

			# List users in a specific organization
			azdo graph user list --organization "MyOrganization"

			# Output the result as JSON
			azdo graph user list --json
		`),
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.projectName = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.organizationName, "organization", "o", "", "Organization name. If not specified, defaults to the default organization")
	cmd.Flags().StringSliceVarP(&opts.subjectTypes, "type", "T", nil, "Subject types filter (comma-separated). If not specified defaults to 'aad'")
	cmd.Flags().StringVarP(&opts.filter, "filter", "F", "", "Filter users by prefix (max 100 results)")
	cmd.Flags().IntVarP(&opts.top, "limit", "L", 20, "Maximum number of users to return (pagination client-side)")
	util.AddJSONFlags(cmd, &opts.exporter, []string{"descriptor", "displayName", "principalName", "mailAddress", "origin"})

	return cmd
}

func runCmd(ctx util.CmdContext, opts *usersListOptions) error {
	io, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	io.StartProgressIndicator()
	defer io.StopProgressIndicator()

	var scope *util.Scope
	switch {
	case opts.projectName != "" && opts.organizationName != "":
		scope, err = util.ParseProjectScope(ctx, fmt.Sprintf("%s/%s", opts.organizationName, opts.projectName))
	case opts.projectName != "":
		scope, err = util.ParseProjectScope(ctx, opts.projectName)
	default:
		org, parseErr := util.ParseOrganizationArg(ctx, opts.organizationName)
		if parseErr != nil {
			return parseErr
		}
		scope = &util.Scope{Organization: org}
	}
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	clientFactory := ctx.ClientFactory()
	client, err := clientFactory.Graph(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	// If a project name is provided, look up its ID, then resolve scopeDescriptor via Graph.Descriptors
	var scopeDescriptor string
	if scope.Project != "" {
		descriptor, _, err := util.ResolveScopeDescriptor(ctx, scope.Organization, scope.Project)
		if err != nil {
			return err
		}
		if descriptor != nil {
			scopeDescriptor = *descriptor
		}
	}

	// Build subject types
	if len(opts.subjectTypes) == 0 {
		opts.subjectTypes = []string{"aad"}
	}

	// Pagination loop using continuation token header
	users := make([]graph.GraphUser, 0, opts.top)
	if opts.filter != "" {
		res, err := client.QuerySubjects(ctx.Context(), graph.QuerySubjectsArgs{
			SubjectQuery: &graph.GraphSubjectQuery{
				Query:           &opts.filter,
				SubjectKind:     &[]string{"User"},
				ScopeDescriptor: &scopeDescriptor,
			},
		})
		if err != nil {
			return err
		}
		for _, s := range *res {
			gu, err := client.GetUser(ctx.Context(), graph.GetUserArgs{
				UserDescriptor: s.Descriptor,
			})
			if err != nil {
				return fmt.Errorf("failed to get user for descriptor %q: %w", valueOr(s.Descriptor), err)
			}
			users = append(users, *gu)
			if len(users) >= opts.top {
				break
			}
		}
	} else {
		cont := types.ToPtr("")
		for len(users) < opts.top {
			res, err := client.ListUsers(ctx.Context(), graph.ListUsersArgs{
				SubjectTypes:      &opts.subjectTypes,
				ContinuationToken: cont,
				ScopeDescriptor:   types.ToPtr(scopeDescriptor),
			})
			if err != nil {
				return err
			}
			if res != nil && res.GraphUsers != nil {
				for _, u := range *res.GraphUsers {
					users = append(users, u)
					if len(users) >= opts.top {
						break
					}
				}
			}
			// Determine next continuation token
			if res == nil || res.ContinuationToken == nil || len(*res.ContinuationToken) == 0 || (*res.ContinuationToken)[0] == "" {
				break
			}
			t := (*res.ContinuationToken)[0]
			cont = &t
		}
	}
	// Sort by DisplayName similar to project list’s stable ordering
	sort.Slice(users, func(i, j int) bool {
		return strings.ToLower(valueOr(users[i].DisplayName)) < strings.ToLower(valueOr(users[j].DisplayName))
	})

	io.StopProgressIndicator()

	if opts.exporter != nil {
		iostreams, err := ctx.IOStreams()
		if err != nil {
			return err
		}
		return opts.exporter.Write(iostreams, users)
	}

	tp, err := ctx.Printer("table")
	if err != nil {
		return err
	}

	// Table/JSON unified via TablePrinter interface-style
	tp.AddColumns("Descriptor", "Display Name", "Principal Name", "Mail")
	for _, u := range users {
		tp.AddField(valueOr(u.Descriptor))
		tp.AddField(valueOr(u.DisplayName), printer.WithTruncate(nil))
		tp.AddField(valueOr(u.PrincipalName))
		tp.AddField(valueOr(u.MailAddress))
		tp.EndRow()
	}
	return tp.Render()
}

func valueOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
