package list

import (
	"fmt"
	"sort"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type opts struct {
	scope          string
	name           string
	repository     string
	repositoryType string
	top            int
	folderPath     string
	queryOrder     string
	maxItems       int
	exporter       util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &opts{}

	cmd := &cobra.Command{
		Use:   "list [ORGANIZATION/]PROJECT",
		Short: "List pipeline definitions",
		Long: heredoc.Doc(`
			List pipeline definitions (YAML or classic) in a project.
		`),
		Example: heredoc.Doc(`
			# List all pipelines in a project
			$ azdo pipelines list "my-project"

			# List pipelines with a specific name
			$ azdo pipelines list "my-project" --name "my-pipeline"

			# List pipelines using a specific repository
			$ azdo pipelines list "my-project" --repository "my-repo"

			# Output as JSON
			$ azdo pipelines list "my-project" --json
		`),
		Aliases: []string{
			"ls",
			"l",
		},
		Args: util.ExactArgs(1, "project argument is required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scope = args[0]
			return runList(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Filter by pipeline name (prefix or exact)")
	cmd.Flags().StringVar(&opts.repository, "repository", "", "Filter by repository name or ID")
	util.StringEnumFlag(cmd, &opts.repositoryType, "repository-type", "", "",
		[]string{"tfsgit", "github"}, "Repository type filter")
	cmd.Flags().IntVar(&opts.top, "top", 0, "Maximum number of definitions to return")
	cmd.Flags().StringVar(&opts.folderPath, "folder-path", "", "Filter by folder path (e.g. \"user1/production\")")
	util.StringEnumFlag(cmd, &opts.queryOrder, "query-order", "", "",
		[]string{"none", "definitionNameAscending", "definitionNameDescending", "lastModifiedAscending", "lastModifiedDescending"},
		"Order of definitions")
	cmd.Flags().IntVar(&opts.maxItems, "max-items", 0, "Optional client-side cap on results")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "name", "path", "revision", "type", "quality", "queueStatus",
		"createdDate", "project", "authoredBy", "latestBuild", "latestCompletedBuild",
		"draftOf", "drafts", "metrics", "queue", "uri", "url", "_links",
	})

	return cmd
}

func runList(cmdCtx util.CmdContext, opts *opts) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if opts.top < 0 {
		return util.FlagErrorf("invalid --top value %d; must be greater than 0", opts.top)
	}
	if opts.maxItems < 0 {
		return util.FlagErrorf("invalid --max-items value %d; must be greater than 0", opts.maxItems)
	}

	scope, err := util.ParseProjectScope(cmdCtx, opts.scope)
	if err != nil {
		return util.FlagErrorf("invalid project argument: %w", err)
	}

	if opts.repository != "" && opts.repositoryType == "" {
		opts.repositoryType = "tfsgit"
	}

	buildClient, err := cmdCtx.ClientFactory().Build(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	var definitions []build.BuildDefinitionReference
	var continuationToken *string

	for {
		args := build.GetDefinitionsArgs{
			Project:           types.ToPtr(scope.Project),
			Name:              types.NotZeroPtrOrNil(opts.name),
			RepositoryId:      types.NotZeroPtrOrNil(opts.repository),
			RepositoryType:    types.NotZeroPtrOrNil(opts.repositoryType),
			Top:               types.PositivePtrOrNil(opts.top),
			Path:              types.NotZeroPtrOrNil(opts.folderPath),
			ContinuationToken: continuationToken,
		}
		if opts.queryOrder != "" {
			order := build.DefinitionQueryOrder(opts.queryOrder)
			args.QueryOrder = &order
		}

		resp, err := buildClient.GetDefinitions(cmdCtx.Context(), args)
		if err != nil {
			return err
		}

		definitions = append(definitions, resp.Value...)

		if opts.maxItems > 0 && len(definitions) >= opts.maxItems {
			definitions = definitions[:opts.maxItems]
			break
		}

		if resp.ContinuationToken == "" {
			break
		}
		continuationToken = &resp.ContinuationToken

		if opts.top > 0 && len(definitions) >= opts.top {
			break
		}
	}

	sort.Slice(definitions, func(i, j int) bool {
		return types.GetValue(definitions[i].Id, 0) < types.GetValue(definitions[j].Id, 0)
	})

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, definitions)
	}

	tp, err := cmdCtx.Printer("table")
	if err != nil {
		return err
	}

	hasDraft := false
	for _, def := range definitions {
		if types.GetValue(def.Quality, "") == "draft" {
			hasDraft = true
			break
		}
	}

	columns := []string{"ID", "PATH", "NAME"}
	if hasDraft {
		columns = append(columns, "DRAFT")
	}
	columns = append(columns, "STATUS", "DEFAULT QUEUE")
	tp.AddColumns(columns...)

	for _, def := range definitions {
		tp.AddField(fmt.Sprintf("%d", types.GetValue(def.Id, 0)))
		tp.AddField(types.GetValue(def.Path, ""))
		tp.AddField(types.GetValue(def.Name, ""))
		if hasDraft {
			if types.GetValue(def.Quality, "") == "draft" {
				tp.AddField("*")
			} else {
				tp.AddField("")
			}
		}
		tp.AddField(string(types.GetValue(def.QueueStatus, "")))
		qName := ""
		if def.Queue != nil {
			qName = types.GetValue(def.Queue.Name, "")
		}
		tp.AddField(qName)
		tp.EndRow()
	}

	return tp.Render()
}
