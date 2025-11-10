package status

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
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

	// Pre-parse constant values and validate config before launching goroutines.
	secNS := uuid.MustParse("5a27515b-ccd7-42c9-84f1-54c998f03866")

	cfg, err := ctx.Config()
	if err != nil {
		return nil, err
	}

	// Launch one goroutine per organization to perform checks concurrently.
	var wg sync.WaitGroup
	wg.Add(len(organizationsToCheck))

	for _, organizationName := range organizationsToCheck {
		organizationName := organizationName // capture
		go func(org string) {
			defer wg.Done()

			status := organizationStatus{
				organizationName: org,
			}

			// Respect context cancellation early.
			select {
			case <-ctx.Context().Done():
				status.err = ctx.Context().Err()
				statusChannel <- status
				return
			default:
			}

			szUrl, err := cfg.Authentication().GetURL(org)
			if err != nil {
				status.err = err
				statusChannel <- status
				return
			}

			parsedURL, err := url.Parse(szUrl)
			if err != nil {
				status.err = err
				statusChannel <- status
				return
			}

			urlOrgName, err := azdo.OrganizationFromURL(parsedURL)
			if err != nil {
				status.err = fmt.Errorf("invalid AzDO url %q for organization %q: %w", szUrl, org, err)
				statusChannel <- status
				return
			}
			if !strings.EqualFold(urlOrgName, org) {
				status.err = fmt.Errorf("url %q of organization %q does not match organization name from URL (%s)", szUrl, org, urlOrgName)
				statusChannel <- status
				return
			}

			// Create client and query a known security namespace to validate auth.
			client, err := ctx.ClientFactory().Security(ctx.Context(), org)
			if err != nil {
				status.err = err
				statusChannel <- status
				return
			}

			// Call the API; if context is cancelled this will return promptly.
			_, err = client.QuerySecurityNamespaces(ctx.Context(), security.QuerySecurityNamespacesArgs{
				SecurityNamespaceId: lo.ToPtr(secNS),
			})
			if err != nil {
				status.err = err
			}

			statusChannel <- status
		}(organizationName)
	}

	// Close the status channel once all workers are done.
	go func() {
		wg.Wait()
		close(statusChannel)
	}()

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
				"%s %s: failed to check authentication status: %s\n", cs.Red("X"), cs.Bold(v.organizationName), v.err.Error())
		} else {
			fmt.Fprintf(iostrms.Out,
				"%s %s: successfully checked authentication status\n", cs.GreenBold("X"), cs.Bold(v.organizationName))
		}
	}
	return nil
}
