package shared

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

// WithCreateCommonOptions already defined in create_common.go

// EndpointTypeConfigurer populates type-specific parts of a service endpoint.
type EndpointTypeConfigurer interface {
	CommandContext() util.CmdContext
	TypeName() string
	Configure(endpoint *serviceendpoint.ServiceEndpoint) error
}

// RunTypedCreate centralizes creation flow for typed service endpoint commands.
func RunTypedCreate(cmd *cobra.Command, args []string, cfg EndpointTypeConfigurer) error {
	cmdCtx := cfg.CommandContext()
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}

	common := cmd.Context().Value("createCommonOptions").(*createCommonOptions)
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectScope(cmdCtx, args[0])
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	projectRef, err := ResolveProjectReference(cmdCtx, scope)
	if err != nil {
		return err
	}

	endpointType := cfg.TypeName()
	owner := "library"

	endpoint := &serviceendpoint.ServiceEndpoint{
		Name:        &common.Name,
		Description: &common.Description,
		Type:        &endpointType,
		Owner:       &owner,
		ServiceEndpointProjectReferences: &[]serviceendpoint.ServiceEndpointProjectReference{{
			ProjectReference: projectRef,
			Name:             &common.Name,
			Description:      &common.Description,
		}},
	}

	err = cfg.Configure(endpoint)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	if common.ValidateSchema {
		if err := ValidateEndpointAgainstMetadata(cmdCtx, scope.Organization, endpoint); err != nil {
			return util.FlagErrorWrap(err)
		}
	}

	client, err := cmdCtx.ClientFactory().ServiceEndpoint(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}

	created, err := client.CreateServiceEndpoint(cmdCtx.Context(), serviceendpoint.CreateServiceEndpointArgs{Endpoint: endpoint})
	if err != nil {
		return fmt.Errorf("failed to create service endpoint: %w", err)
	}

	if common.WaitReady {
		created, err = WaitForReady(cmdCtx.Context(), client, scope.Project, created, common.Timeout)
		if err != nil {
			return err
		}
	}

	if common.ValidateConnection {
		if err := TestConnection(cmdCtx, client, scope.Organization, scope.Project, created, common.Timeout); err != nil {
			return err
		}
	}

	if common.GrantAllPipelines {
		projectID := types.GetValue(projectRef.Id, uuid.Nil)
		if projectID == uuid.Nil {
			return fmt.Errorf("project reference missing ID")
		}
		endpointID := types.GetValue(created.Id, uuid.Nil)
		if endpointID == uuid.Nil {
			return fmt.Errorf("service endpoint create response missing ID")
		}

		if err := SetAllPipelinesAccessToEndpoint(cmdCtx, scope.Organization, projectID, endpointID, true, func() error {
			return client.DeleteServiceEndpoint(cmdCtx.Context(), serviceendpoint.DeleteServiceEndpointArgs{
				EndpointId: &endpointID,
				ProjectIds: &[]string{projectID.String()},
			})
		}); err != nil {
			return err
		}
	}

	ios.StopProgressIndicator()

	// redact secrets before output
	RedactSecrets(created)
	return Output(cmdCtx, created, common.Exporter)
}
