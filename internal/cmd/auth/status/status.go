package status

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

type statusOptions struct {
	organizationName string
}

func NewCmdStatus(ctx util.CmdContext) *cobra.Command {
	opts := &statusOptions{}

	cmd := &cobra.Command{
		Use:   "status [organization]",
		Args:  cobra.MaximumNArgs(1),
		Short: "View authentication status",
		Long: heredoc.Doc(`Verifies and displays information about your authentication state.

			This command will test your authentication state for each Azure DevOps organization that azdo knows about and
			report any issues.
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.organizationName = args[0]
			}
			return statusRun(ctx, opts)
		},
	}

	return cmd
}

type organizationStatus struct {
	organizationName string
	err              error
}

func fetchOrganizationStates(ctx util.CmdContext, organizationsToCheck []string) (<-chan organizationStatus, error) {
	statusChannel := make(chan organizationStatus)

	go func(channel chan<- organizationStatus) error {
		for _, organizationName := range organizationsToCheck {
			client, err := ctx.ConnectionFactory().Security(ctx.Context(), organizationName)
			if err != nil {
				return err
			}

			_, err = client.QuerySecurityNamespaces(ctx.Context(), security.QuerySecurityNamespacesArgs{SecurityNamespaceId: lo.ToPtr(uuid.MustParse("5a27515b-ccd7-42c9-84f1-54c998f03866"))})

			status := organizationStatus{
				organizationName: organizationName,
			}
			if err != nil {
				status.err = err
			}

			channel <- status
		}

		close(statusChannel)
		return nil
	}(statusChannel) //nolint:golint,errcheck

	return statusChannel, nil
}

func statusRun(ctx util.CmdContext, opts *statusOptions) (err error) {
	cfg, err := ctx.Config()
	if err != nil {
		return err
	}
	authCfg := cfg.Authentication()

	organizations := authCfg.GetOrganizations()

	iostrms, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	stderr := iostrms.ErrOut
	cs := iostrms.ColorScheme()

	if len(organizations) == 0 {
		fmt.Fprintf(
			stderr,
			"You are not logged into any Azure DevOps organizations. Run %s to authenticate.\n",
			cs.Bold("azdo auth login"),
		)

		return util.ErrSilent
	}

	organizationsToCheck := organizations

	if opts.organizationName != "" {
		if !lo.Contains(organizations, opts.organizationName) {
			fmt.Fprintf(
				stderr,
				"You are not logged the Azure DevOps organization %s. Run %s to authenticate.\n",
				cs.Red(opts.organizationName), cs.Bold("azdo auth login"),
			)
			return util.ErrSilent
		}
		organizationsToCheck = []string{opts.organizationName}
	}

	organizationStatusResults := []organizationStatus{}

	iostrms.StartProgressIndicator()
	organizationStatusChannel, err := fetchOrganizationStates(ctx, organizationsToCheck)
	if err != nil {
		return err
	}

	for {
		result, ok := <-organizationStatusChannel
		if !ok {
			break
		}
		organizationStatusResults = append(organizationStatusResults, result)
	}

	iostrms.StopProgressIndicator()

	for _, v := range organizationStatusResults {
		if v.err != nil {
			fmt.Fprintf(iostrms.Out,
				"%s %s: failed to check authentication status\n", cs.Red("X"), cs.Bold(v.organizationName))
		} else {
			fmt.Fprintf(iostrms.Out,
				"%s %s: successfully checked authentication status\n", cs.GreenBold("X"), cs.Bold(v.organizationName))
		}
	}
	return nil
}
