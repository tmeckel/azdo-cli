package list

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
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

	cfg, err := ctx.Config()
	if err != nil {
		return err
	}

	var organizationName string
	if opts.organizationName != "" {
		organizationName = opts.organizationName
	} else {
		organizationName, err = cfg.Authentication().GetDefaultOrganization()
		if err != nil {
			return err
		}
	}
	if organizationName == "" {
		return util.FlagErrorf("no organization specified")
	}

	clientFactory := ctx.ClientFactory()
	client, err := clientFactory.Graph(ctx.Context(), organizationName)
	if err != nil {
		return err
	}

	// If a project name is provided, look up its ID, then resolve scopeDescriptor via Graph.Descriptors
	var scopeDescriptor string
	var cont *string
	if opts.projectName != "" {
		coreClient, err := clientFactory.Core(ctx.Context(), organizationName)
		if err != nil {
			return err
		}

		found := false
		var pid uuid.UUID
		for !found {
			args := core.GetProjectsArgs{}
			if cont != nil {
				n, err := strconv.Atoi(*cont)
				if err != nil {
					return fmt.Errorf("failed to parse continuation token %q to list projects: %w", *cont, err)
				}
				args.ContinuationToken = &n
			}
			res, err := coreClient.GetProjects(ctx.Context(), args)
			if err != nil {
				return err
			}
			if res == nil || res.Value == nil || len(res.Value) == 0 {
				break
			}
			for _, p := range res.Value {
				if p.Name != nil && strings.EqualFold(*p.Name, opts.projectName) {
					pid = *p.Id
					found = true
					break
				}
			}
			// Determine next continuation token
			if res.ContinuationToken == "" {
				break
			}
			cont = &res.ContinuationToken
		}
		if !found {
			return fmt.Errorf("project %q not found in organization %q", opts.projectName, organizationName)
		}

		// Resolve descriptor
		desc, err := client.GetDescriptor(ctx.Context(), graph.GetDescriptorArgs{StorageKey: &pid})
		if err != nil {
			return fmt.Errorf("failed to resolve project scope descriptor: %w", err)
		}
		if desc == nil || desc.Value == nil || *desc.Value == "" {
			return fmt.Errorf("no scope descriptor for project %q", opts.projectName)
		}
		scopeDescriptor = *desc.Value
	}

	// Build subject types
	var subj *[]string
	if len(opts.subjectTypes) > 0 {
		s := opts.subjectTypes
		subj = &s
	} else {
		subj = &[]string{"aad"}
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
		cont = nil
		for len(users) < opts.top {
			res, err := client.ListUsers(ctx.Context(), graph.ListUsersArgs{
				SubjectTypes:      subj,
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
