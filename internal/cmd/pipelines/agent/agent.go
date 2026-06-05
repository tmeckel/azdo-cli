package agent

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/pipelines/agent/show"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage Azure DevOps pipeline agents",
		Long: heredoc.Doc(`
			Manage Azure DevOps pipeline agents. Agents are the compute targets
			that run build, release, and other pipeline jobs. Each agent belongs
			to an agent pool, which is identified by name or numeric ID.

			Targets are specified in POOL/AGENT format where each component can
			be a numeric ID or a name. An optional organization prefix can be
			included: [ORGANIZATION/]POOL/AGENT.
		`),
		Example: heredoc.Doc(`
			# Show agent by pool ID and agent ID
			azdo pipelines agent show 1/42

			# Show agent by pool name and agent name
			azdo pipelines agent show 'Default/my-agent'

			# Show agent in a different organization
			azdo pipelines agent show 'myorg/1/42'

			# Show agent with system and user capabilities
			azdo pipelines agent show 1/42 --include-capabilities
		`),
		Aliases: []string{"agents", "a"},
	}

	cmd.AddCommand(show.NewCmd(ctx))
	return cmd
}
