package delete

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/boards/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type deleteOptions struct {
	scopeArg     string
	path         string
	reclassifyID *int
	yes          bool
	exporter     util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &deleteOptions{}

	cmd := &cobra.Command{
		Use:   "delete [ORGANIZATION/]PROJECT --path <path>",
		Short: "Delete an iteration from a project.",
		Long: heredoc.Doc(`
			Delete an iteration (sprint) from a project. The command prompts for
			confirmation unless --yes is supplied. Use --reclassify-id to move any
			work items to another node before deletion; the Azure DevOps REST API
			rejects deletes while a node is still in use unless work items are
			reclassified first.
		`),
		Example: heredoc.Doc(`
			# Delete a top-level iteration
			azdo boards iteration project delete Fabrikam --path "Sprint 1" --yes

			# Delete a nested iteration with a confirmation prompt
			azdo boards iteration project delete Fabrikam --path "Release 2025/Sprint 1"

			# Reclassify work items to node 42 before deletion
			azdo boards iteration project delete Fabrikam --path "Sprint 1" \
				--reclassify-id 42 --yes

			# Emit JSON
			azdo boards iteration project delete Fabrikam --path "Sprint 1" --reclassify-id 42 --json
		`),
		Aliases: []string{"d", "del", "rm"},
		Args:    util.ExactArgs(1, "project argument required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.scopeArg = args[0]
			return runDelete(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.path, "path", "", "Path of the iteration to delete (under /Iteration, leading /Iteration stripped).")
	util.NilIntFlag(cmd, &opts.reclassifyID, "reclassify-id", "r", "ID of the target node to which work items should be moved before deletion.")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip the confirmation prompt.")
	_ = cmd.MarkFlagRequired("path")
	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"deleted", "path", "reclassifyId",
	})

	return cmd
}

func runDelete(ctx util.CmdContext, opts *deleteOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	if parts := strings.Split(strings.TrimSpace(opts.scopeArg), "/"); len(parts) > 2 {
		return util.FlagErrorf("invalid project scope %q: expected [ORGANIZATION/]PROJECT", opts.scopeArg)
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

	if !opts.yes {
		if !ios.CanPrompt() {
			return util.FlagErrorf("--yes required when not running interactively")
		}
		ios.StopProgressIndicator()
		prompter, err := ctx.Prompter()
		if err != nil {
			return err
		}
		prompt := fmt.Sprintf("Delete iteration %q from project %s/%s?", nodePath, scope.Organization, scope.Project)
		confirmed, err := prompter.Confirm(prompt, false)
		if err != nil {
			return err
		}
		if !confirmed {
			zap.L().Debug(
				"iteration deletion canceled by user",
				zap.String("organization", scope.Organization),
				zap.String("project", scope.Project),
				zap.String("path", nodePath),
			)
			return util.ErrCancel
		}
		ios.StartProgressIndicator()
	}

	zap.L().Debug(
		"deleting iteration",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("path", nodePath),
	)

	wit, err := ctx.ClientFactory().WorkItemTracking(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to get classification client: %w", err)
	}

	args := workitemtracking.DeleteClassificationNodeArgs{
		Project:        types.ToPtr(scope.Project),
		StructureGroup: types.ToPtr(workitemtracking.TreeStructureGroupValues.Iterations),
		Path:           types.ToPtr(nodePath),
	}
	if opts.reclassifyID != nil {
		args.ReclassifyId = opts.reclassifyID
	}

	if err := wit.DeleteClassificationNode(ctx.Context(), args); err != nil {
		return fmt.Errorf("failed to delete iteration: %w", err)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		if opts.reclassifyID != nil {
			return opts.exporter.Write(ios, map[string]any{
				"deleted":      true,
				"path":         nodePath,
				"reclassifyId": *opts.reclassifyID,
			})
		}
		return opts.exporter.Write(ios, map[string]any{
			"deleted": true,
			"path":    nodePath,
		})
	}

	fmt.Fprintf(ios.Out, "Deleted iteration: %s\n", nodePath)
	if opts.reclassifyID != nil {
		fmt.Fprintf(ios.Out, "Reclassified work items to: %d\n", *opts.reclassifyID)
	}
	return nil
}
