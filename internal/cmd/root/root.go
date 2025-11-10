package root

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/auth"
	"github.com/tmeckel/azdo-cli/internal/cmd/boards"
	"github.com/tmeckel/azdo-cli/internal/cmd/config"
	"github.com/tmeckel/azdo-cli/internal/cmd/graph"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr"
	"github.com/tmeckel/azdo-cli/internal/cmd/project"
	"github.com/tmeckel/azdo-cli/internal/cmd/repo"
	"github.com/tmeckel/azdo-cli/internal/cmd/security"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	versionCmd "github.com/tmeckel/azdo-cli/internal/cmd/version"
	"github.com/tmeckel/azdo-cli/internal/validation"
)

type AuthError struct {
	err error
}

func (ae *AuthError) Error() string {
	return ae.err.Error()
}

func NewCmdRoot(ctx util.CmdContext, version, buildDate string) (*cobra.Command, error) {
	cfg, err := ctx.Config()
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration: %w", err)
	}
	iostrms, err := ctx.IOStreams()
	if err != nil {
		return nil, fmt.Errorf("failed to get IOStreams: %w", err)
	}

	cmd := &cobra.Command{
		Use:   "azdo <command> <subcommand> [flags]",
		Short: "Azure DevOps CLI",
		Long:  `Work seamlessly with Azure DevOps from the command line.`,
		Example: heredoc.Doc(`
		$ azdo project list
		$ azdo repo clone myorg/myrepo
	`),
		Annotations: map[string]string{
			"versionInfo": versionCmd.Format(version, buildDate),
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// require that the user is authenticated before running most commands
			if util.IsAuthCheckEnabled(cmd) && !util.CheckAuth(cfg) {
				return &AuthError{}
			}
			return nil
		},
	}

	cmd.PersistentFlags().Bool("help", false, "Show help for command")

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	cmd.Flags().Bool("version", false, "Show azdo version")

	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		rootHelpFunc(iostrms, c, args)
	})
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		return rootUsageFunc(iostrms.ErrOut, c)
	})

	cmd.SetFlagErrorFunc(rootFlagErrorFunc)

	cmd.AddGroup(&cobra.Group{
		ID:    "core",
		Title: "Core commands",
	})

	cmd.AddGroup(&cobra.Group{
		ID:    "security",
		Title: "Security commands",
	})

	cmd.AddCommand(versionCmd.NewCmdVersion(ctx, version, buildDate))
	cmd.AddCommand(auth.NewCmdAuth(ctx))
	cmd.AddCommand(config.NewCmdConfig(ctx))
	cmd.AddCommand(project.NewCmdProject(ctx))
	cmd.AddCommand(repo.NewCmdRepo(ctx))
	cmd.AddCommand(pr.NewCmdPR(ctx))
	cmd.AddCommand(graph.NewCmdGraph(ctx))
	cmd.AddCommand(security.NewCmd(ctx))
	cmd.AddCommand(serviceendpoint.NewCmd(ctx))
	cmd.AddCommand(boards.NewCmd(ctx))

	// Help topics
	var referenceCmd *cobra.Command
	for _, ht := range HelpTopics {
		helpTopicCmd := NewCmdHelpTopic(iostrms, ht)
		cmd.AddCommand(helpTopicCmd)

		// See bottom of the function for why we explicitly care about the reference cmd
		if ht.name == "reference" {
			referenceCmd = helpTopicCmd
		}
	}

	// Aliases
	aliases := cfg.Aliases()
	validAliasName := validation.ValidAliasNameFunc(cmd)
	validAliasExpansion := validation.ValidAliasExpansionFunc(cmd)
	for k, v := range aliases.All() {
		aliasName := k
		aliasValue := v
		if validAliasName(aliasName) && validAliasExpansion(aliasValue) {
			split, _ := shlex.Split(aliasName)
			parentCmd, parentArgs, _ := cmd.Find(split)
			if !parentCmd.ContainsGroup("alias") {
				parentCmd.AddGroup(&cobra.Group{
					ID:    "alias",
					Title: "Alias commands",
				})
			}
			if strings.HasPrefix(aliasValue, "!") {
				shellAliasCmd, err := NewCmdShellAlias(ctx, parentArgs[0], aliasValue)
				if err != nil {
					return nil, err
				}
				parentCmd.AddCommand(shellAliasCmd)
			} else {
				aliasCmd, err := NewCmdAlias(ctx, parentArgs[0], aliasValue)
				if err != nil {
					return nil, err
				}
				split, _ := shlex.Split(aliasValue)
				child, _, _ := cmd.Find(split)
				aliasCmd.SetUsageFunc(func(_ *cobra.Command) error {
					return rootUsageFunc(iostrms.ErrOut, child)
				})
				aliasCmd.SetHelpFunc(func(_ *cobra.Command, args []string) {
					rootHelpFunc(iostrms, child, args)
				})
				parentCmd.AddCommand(aliasCmd)
			}
		}
	}

	util.DisableAuthCheck(cmd)

	// The reference command produces paged output that displays information on every other command.
	// Therefore, we explicitly set the Long text and HelpFunc here after all other commands are registered.
	// We experimented with producing the paged output dynamically when the HelpFunc is called but since
	// doc generation makes use of the Long text, it is simpler to just be explicit here that this command
	// is special.
	referenceCmd.Long = stringifyReference(cmd)
	referenceCmd.SetHelpFunc(longPager(iostrms))

	return cmd, nil
}
