package shared

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// RunTypedUpdate centralizes update flow for typed service endpoint commands.
func RunTypedUpdate(cmd *cobra.Command, args []string, cfg EndpointTypeConfigurer) error {
	cmdCtx := cfg.CommandContext()
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}

	common := cmd.Context().Value("updateCommonOptions").(*updateCommonOptions)
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	// 1. Parse scope
	scope, err := util.ParseProjectTargetWithDefaultOrganization(cmdCtx, args[0])
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	client, err := cmdCtx.ClientFactory().ServiceEndpoint(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}

	// 2. Find existing endpoint
	endpoint, err := FindServiceEndpoint(cmdCtx, client, scope.Project, scope.Target)
	if err != nil {
		if errors.Is(err, ErrEndpointNotFound) {
			ios.StopProgressIndicator()
			cs := ios.ColorScheme()
			fmt.Fprintf(ios.Out, "%s Service endpoint %q was not found in %s/%s.\n", cs.WarningIcon(), scope.Target, scope.Organization, scope.Project)
			return nil
		}
		return err
	}

	// 3. Update common fields
	if cmd.Flags().Changed("name") {
		endpoint.Name = &common.Name
	}
	if cmd.Flags().Changed("description") {
		endpoint.Description = &common.Description
	}

	// 4. Update type specific fields
	err = cfg.Configure(endpoint)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	// 5. Validate schema
	if common.ValidateSchema {
		if err := ValidateEndpointAgainstMetadata(cmdCtx, scope.Organization, endpoint); err != nil {
			return util.FlagErrorWrap(err)
		}
	}

	// 6. Execute Update
	updated, err := client.UpdateServiceEndpoint(cmdCtx.Context(), serviceendpoint.UpdateServiceEndpointArgs{
		Endpoint:   endpoint,
		EndpointId: endpoint.Id,
	})
	if err != nil {
		return fmt.Errorf("failed to update service endpoint: %w", err)
	}

	// 7. Wait if requested
	if common.WaitReady {
		updated, err = WaitForReady(cmdCtx.Context(), client, scope.Project, updated, common.Timeout)
		if err != nil {
			return err
		}
	}

	// 8. Validate connection
	if common.ValidateConnection {
		if err := TestConnection(cmdCtx, client, scope.Organization, scope.Project, updated, common.Timeout); err != nil {
			return err
		}
	}

	// 9. Pipeline permissions
	if cmd.Flags().Changed("grant-permission-to-all-pipelines") {
		projectRef, err := ResolveProjectReference(cmdCtx, &scope.Scope)
		if err != nil {
			return err
		}
		projectID := types.GetValue(projectRef.Id, uuid.Nil)
		endpointID := types.GetValue(updated.Id, uuid.Nil)

		if err := SetAllPipelinesAccessToEndpoint(cmdCtx, scope.Organization, projectID, endpointID, common.GrantAllPipelines, nil); err != nil {
			return err
		}
	}

	ios.StopProgressIndicator()

	// redact secrets before output
	RedactSecrets(updated)
	return Output(cmdCtx, updated, common.Exporter)
}
