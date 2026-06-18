package show

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spewerspew/spew"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type showOptions struct {
	scopeArg        string
	path            string
	depth           int
	includeChildren bool
	raw             bool
	exporter        util.Exporter
}

//go:embed show.tpl
var showTpl string

type templateData struct {
	Node            *workitemtracking.WorkItemClassificationNode
	IncludeChildren bool
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &showOptions{}

	cmd := &cobra.Command{
		Use:   "show [ORGANIZATION/]PROJECT",
		Short: "Show an iteration in a project.",
		Long: heredoc.Doc(`
			Display the details of a single iteration (sprint) node in a project.
			The iteration is identified by its fully-qualified path under /Iteration.
		`),
		Example: heredoc.Doc(`
			# Show a top-level iteration
			azdo boards iteration project show Fabrikam --path "Sprint 1"

			# Show a nested iteration
			azdo boards iteration project show myorg/Fabrikam --path "Release 2025/Sprint 1"

			# Include child nodes in the template output
			azdo boards iteration project show Fabrikam --path "Release 2025" --include-children

			# Emit the raw SDK node as JSON
			azdo boards iteration project show Fabrikam --path "Sprint 1" --json
		`),
		Aliases: []string{"view", "status"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runShow(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.path, "path", "", "Iteration path under /Iteration (required).")
	cmd.Flags().IntVar(&opts.depth, "depth", 0, "Depth of child nodes to fetch (0-10).")
	cmd.Flags().BoolVar(&opts.includeChildren, "include-children", false, "Include child nodes in the template output.")
	cmd.Flags().BoolVarP(&opts.raw, "raw", "r", false, "Dump the raw SDK node to stderr.")
	_ = cmd.MarkFlagRequired("path")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"id", "identifier", "name", "path", "structureType",
		"hasChildren", "attributes", "url", "_links", "children",
	})

	return cmd
}

func runShow(ctx util.CmdContext, opts *showOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if opts.depth < 0 || opts.depth > 10 {
		return util.FlagErrorf("--depth must be between 0 and 10")
	}

	scope, err := util.ParseProjectScope(ctx, opts.scopeArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	rawPath := strings.TrimSpace(opts.path)
	if rawPath == "" {
		return util.FlagErrorf("--path must not be empty")
	}
	nodePath, err := shared.BuildClassificationPath(scope.Project, true, "Iteration", rawPath)
	if err != nil {
		return util.FlagErrorf("invalid --path: %w", err)
	}
	if nodePath == "" {
		return util.FlagErrorf("--path must reference a child of /Iteration, not the iteration root")
	}

	wit, err := ctx.ClientFactory().WorkItemTracking(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to get classification client: %w", err)
	}

	zap.L().Debug(
		"fetching iteration",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("path", nodePath),
		zap.Int("depth", opts.depth),
	)

	args := workitemtracking.GetClassificationNodeArgs{
		Project:        types.ToPtr(scope.Project),
		StructureGroup: types.ToPtr(workitemtracking.TreeStructureGroupValues.Iterations),
		Path:           types.ToPtr(nodePath),
		Depth:          types.ToPtr(opts.depth),
	}
	res, err := wit.GetClassificationNode(ctx.Context(), args)
	if err != nil {
		return fmt.Errorf("failed to get iteration: %w", err)
	}
	if res == nil {
		return fmt.Errorf("iteration node is nil")
	}

	ios.StopProgressIndicator()

	if opts.raw {
		spew.NewDefaultConfig().Fdump(ios.ErrOut, res)
		return nil
	}

	if opts.exporter != nil {
		return opts.exporter.Write(ios, res)
	}

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled(),
	).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"hasText": template.HasText,
			"s":       template.StringOrEmpty,
			"bool":    template.BoolString,
			"int":     func(v *int) string { return strconv.Itoa(types.GetValue(v, 0)) },
			"uuid":    template.UUIDString,
		})
	if err := t.Parse(showTpl); err != nil {
		return err
	}

	return t.ExecuteData(templateData{
		Node:            res,
		IncludeChildren: opts.includeChildren,
	})
}
