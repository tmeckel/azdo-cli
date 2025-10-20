package list

import (
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	target       string
	filter       string
	exporter     util.Exporter
	subjectTypes []string
}

type securityGroup struct {
	ID            *string `json:"id,omitempty"`
	Name          *string `json:"name,omitempty"`
	Description   *string `json:"description,omitempty"`
	MailAddress   *string `json:"mailAddress,omitempty"`
	PrincipalName *string `json:"principalName,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	o := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION[/PROJECT]]",
		Short: "List security groups",
		Long:  "List all security groups within a given project or organization.",
		Example: heredoc.Docf(`
			# List all security groups in the default organization
			azdo security group list

			# List all groups matching a regex pattern
			azdo security group list --filter 'dev.*team'

			# List groups filtering by regex and restricting by subject types
			azdo security group list --filter '-qa$' --subject-types vssgp,aadgp
		`),
		Args: cobra.MaximumNArgs(1),
		Aliases: []string{
			"ls",
			"l",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				o.target = args[0]
			}
			return runCommand(ctx, o)
		},
	}

	cmd.Flags().StringVarP(&o.filter, "filter", "f", "", "Case-insensitive regex to filter groups by name (e.g. 'dev.*team'). Invalid patterns will result in an error")
	cmd.Flags().StringSliceVar(&o.subjectTypes, "subject-types", nil, "List of subject types to include (comma-separated). Values must not be empty or contain only whitespace.")
	util.AddJSONFlags(cmd, &o.exporter, []string{"id", "name", "description", "mailAddress", "principalName"})

	return cmd
}

func runCommand(ctx util.CmdContext, o *opts) error {
	zap.L().Sugar().Debug("Starting security group list command")

	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	var re *regexp.Regexp
	if o.filter != "" {
		var err error
		re, err = regexp.Compile("(?i)" + o.filter)
		if err != nil {
			return util.FlagErrorf("invalid filter regex: %v", err)
		}
		zap.L().Sugar().Debugf("Using case-insensitive regex for filtering: %s", o.filter)
	}

	// Validate subject-types values
	if len(o.subjectTypes) > 0 {
		for _, st := range o.subjectTypes {
			if strings.TrimSpace(st) == "" {
				return util.FlagErrorf("subject-types contains empty or whitespace-only value")
			}
		}
	}
	scope, err := util.ParseScope(ctx, o.target)
	if err != nil {
		return err
	}

	organization := scope.Organization
	project := scope.Project

	zap.L().Sugar().Debugf("Organization: %s", organization)
	graphClient, err := ctx.ClientFactory().Graph(ctx.Context(), organization)
	if err != nil {
		return fmt.Errorf("failed to get graph client: %w", err)
	}
	zap.L().Sugar().Debug("Graph client created")

	scopeDescriptor, _, err := util.ResolveScopeDescriptor(ctx, organization, project)
	if err != nil {
		return err
	}
	if scopeDescriptor != nil {
		zap.L().Sugar().Debugf("Project descriptor: %s", types.GetValue(scopeDescriptor, ""))
	}

	zap.L().Sugar().Debug("Starting group fetch loop")
	var allGroups []graph.GraphGroup
	var continuationToken *string
	for {
		zap.L().Sugar().Debugf("Loop iteration with continuationToken: %v", continuationToken)
		args := graph.ListGroupsArgs{}
		if len(o.subjectTypes) > 0 {
			args.SubjectTypes = &o.subjectTypes
		}
		if scopeDescriptor != nil {
			args.ScopeDescriptor = scopeDescriptor
		}
		if continuationToken != nil {
			args.ContinuationToken = continuationToken
		}

		response, err := graphClient.ListGroups(ctx.Context(), args)
		if err != nil {
			return fmt.Errorf("failed to list groups: %w", err)
		}
		zap.L().Sugar().Debugf("Fetched %d groups in this batch", len(*response.GraphGroups))

		if o.filter == "" {
			allGroups = append(allGroups, *response.GraphGroups...)
		} else {
			for _, g := range *response.GraphGroups {
				if re != nil && re.MatchString(types.GetValue(g.DisplayName, "")) {
					allGroups = append(allGroups, g)
					zap.L().Sugar().Debugf("Group matched filter and added: %s", types.GetValue(g.DisplayName, ""))
				}
			}
		}

		if response.ContinuationToken == nil || len(*response.ContinuationToken) == 0 || (*response.ContinuationToken)[0] == "" {
			zap.L().Sugar().Debug("No continuation token, ending loop")
			break
		}

		zap.L().Sugar().Debugf("Continuation token found, will fetch next batch: %s", (*response.ContinuationToken)[0])
		continuationToken = &(*response.ContinuationToken)[0]
	}
	zap.L().Sugar().Debugf("Total groups fetched: %d", len(allGroups))

	ios.StopProgressIndicator()

	if o.exporter != nil {
		// JSON output
		var results []securityGroup
		for _, g := range allGroups {
			results = append(results, securityGroup{
				ID:            g.Descriptor,
				Name:          g.DisplayName,
				Description:   g.Description,
				MailAddress:   g.MailAddress,
				PrincipalName: g.PrincipalName,
			})
		}
		return o.exporter.Write(ios, results)
	}

	// Table output
	tp, err := ctx.Printer("table")
	if err != nil {
		return err
	}

	tp.AddColumns("ID", "DisplayName", "Description", "Principal Name")
	tp.EndRow()
	for _, g := range allGroups {
		tp.AddField(types.GetValue(g.Descriptor, ""))
		tp.AddField(types.GetValue(g.DisplayName, ""))
		tp.AddField(types.GetValue(g.Description, ""))
		tp.AddField(types.GetValue(g.PrincipalName, ""))
		tp.EndRow()
	}
	return tp.Render()
}
