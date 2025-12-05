package azurerm

import (
	"errors"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/prompter"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type createOptions struct {
	project string

	name                          string
	description                   string
	authenticationScheme          string
	servicePrincipalID            string
	servicePrincipalKey           string
	servicePrincipalCertificate   string
	certificatePath               string
	tenantID                      string
	subscriptionID                string
	subscriptionName              string
	managementGroupID             string
	managementGroupName           string
	environment                   string
	resourceGroup                 string
	serverURL                     string
	serviceEndpointCreationMode   string
	grantPermissionToAllPipelines bool

	yes      bool
	exporter util.Exporter
}

const (
	// Authentication Schemes
	AuthSchemeServicePrincipal           = "ServicePrincipal"
	AuthSchemeManagedServiceIdentity     = "ManagedServiceIdentity"
	AuthSchemeWorkloadIdentityFederation = "WorkloadIdentityFederation"

	// Creation Modes
	CreationModeManual    = "Manual"
	CreationModeAutomatic = "Automatic"

	// Scope Levels
	ScopeLevelSubscription    = "Subscription"
	ScopeLevelResourceGroup   = "ResourceGroup"
	ScopeLevelManagementGroup = "ManagementGroup"
)

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &createOptions{}

	cmd := &cobra.Command{
		Use:   "azurerm [ORGANIZATION/]PROJECT --name <name> --authentication-scheme <scheme> [flags]",
		Short: "Create an Azure Resource Manager service connection",
		Long: heredoc.Doc(`
			Create an Azure Resource Manager service connection.
			This command is modeled after the Azure DevOps Terraform Provider's implementation for creating azurerm service endpoints.
		`),
		Example: heredoc.Doc(`
			# Service Principal with a secret
			azdo service-endpoint create azurerm my-org/my-project \
					--name "My AzureRM SPN Secret Connection" \
					--authentication-scheme ServicePrincipal \
					--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
					--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
					--service-principal-key "my-service-principal-secret" \
					--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
					--subscription-name "My Azure Subscription" \
					--resource-group "my-resource-group" \
					--description "Service Connection for my AzureRM resources"

			# Service Principal with a certificate
			azdo service-endpoint create azurerm my-org/my-project \
					--name "My AzureRM SPN Cert Connection" \
					--authentication-scheme ServicePrincipal \
					--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
					--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
					--certificate-path "/path/to/my-cert.pem" \
					--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
					--subscription-name "My Azure Subscription" \
					--description "Certificate-based Service Connection"

			# Managed Service Identity
			azdo service-endpoint create azurerm my-org/my-project \
					--name "My AzureRM MSI Connection" \
					--authentication-scheme ManagedServiceIdentity \
					--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
					--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
					--subscription-name "My Azure Subscription" \
					--description "MSI Service Connection"

			# Workload Identity Federation (Manual mode, with existing Service Principal)
			azdo service-endpoint create azurerm my-org/my-project \
					--name "My AzureRM WIF Manual Connection" \
					--authentication-scheme WorkloadIdentityFederation \
					--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
					--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
					--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
					--subscription-name "My Azure Subscription" \
					--description "WIF Manual Service Connection"

			# Workload Identity Federation (Automatic mode, Azure DevOps creates Service Principal)
			azdo service-endpoint create azurerm my-org/my-project \
					--name "My AzureRM WIF Automatic Connection" \
					--authentication-scheme WorkloadIdentityFederation \
					--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
					--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
					--subscription-name "My Azure Subscription" \
					--description "WIF Automatic Service Connection"

			# Service Principal with Management Group Scope
			azdo service-endpoint create azurerm my-org/my-project \
					--name "My AzureRM MGMT Group Connection" \
					--authentication-scheme ServicePrincipal \
					--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
					--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
					--service-principal-key "my-service-principal-secret" \
					--management-group-id "my-mgmt-group-id" \
					--management-group-name "My Management Group" \
					--description "Service Connection scoped to a Management Group"

			# Azure Stack Environment
			azdo service-endpoint create azurerm my-org/my-project \
					--name "My AzureStack Connection" \
					--authentication-scheme ServicePrincipal \
					--tenant-id "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
					--service-principal-id "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy" \
					--service-principal-key "my-service-principal-secret" \
					--subscription-id "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" \
					--subscription-name "My Azure Stack Subscription" \
					--environment AzureStack \
					--server-url "https://management.myazurestack.com/" \
					--description "Service Connection for Azure Stack"
		`),
		Args: cobra.ExactArgs(1),
		Aliases: []string{
			"cr",
			"c",
			"new",
			"n",
			"add",
			"a",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.project = args[0]
			return runCreate(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.name, "name", "", "Name of the service endpoint")
	cmd.Flags().StringVar(&opts.description, "description", "", "Description for the service endpoint")
	util.StringEnumFlag(cmd, &opts.authenticationScheme, "authentication-scheme", "", AuthSchemeServicePrincipal,
		[]string{AuthSchemeServicePrincipal, AuthSchemeManagedServiceIdentity, AuthSchemeWorkloadIdentityFederation},
		"Authentication scheme")
	cmd.Flags().StringVar(&opts.tenantID, "tenant-id", "", "Azure tenant ID (e.g., GUID)")
	cmd.Flags().StringVar(&opts.subscriptionID, "subscription-id", "", "Azure subscription ID (e.g., GUID)")
	cmd.Flags().StringVar(&opts.subscriptionName, "subscription-name", "", "Azure subscription name")
	cmd.Flags().StringVar(&opts.managementGroupID, "management-group-id", "", "Azure management group ID")
	cmd.Flags().StringVar(&opts.managementGroupName, "management-group-name", "", "Azure management group name")
	cmd.Flags().StringVar(&opts.resourceGroup, "resource-group", "", "Name of the resource group (for subscription-level scope)")
	cmd.Flags().StringVar(&opts.servicePrincipalID, "service-principal-id", "", "Service principal/application ID (e.g., GUID)")
	cmd.Flags().StringVar(&opts.servicePrincipalKey, "service-principal-key", "", "Service principal key (secret value)")
	cmd.Flags().StringVar(&opts.certificatePath, "certificate-path", "", "Path to service principal certificate file (PEM format)")
	util.StringEnumFlag(cmd, &opts.environment, "environment", "", "AzureCloud",
		[]string{"AzureCloud", "AzureChinaCloud", "AzureUSGovernment", "AzureGermanCloud", "AzureStack"},
		"Azure environment")
	cmd.Flags().StringVar(&opts.serverURL, "server-url", "", "Azure Stack Resource Manager base URL. Required if --environment is AzureStack.")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&opts.grantPermissionToAllPipelines, "grant-permission-to-all-pipelines", false, "Grant access permission to all pipelines to use the service connection")

	util.AddJSONFlags(cmd, &opts.exporter, []string{"id", "name", "type", "url", "description", "authorization"})

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("authentication-scheme")

	return cmd
}

func runCreate(ctx util.CmdContext, opts *createOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	p, err := ctx.Prompter()
	if err != nil {
		return err
	}

	scope, err := util.ParseProjectScope(ctx, opts.project)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	if err := validateOpts(opts, ios, p); err != nil {
		return util.FlagErrorWrap(err)
	}

	if !opts.yes {
		ok, err := p.Confirm("This will create credentials in Azure DevOps. Continue?", false)
		if err != nil {
			return err
		}
		if !ok {
			return util.ErrCancel
		}
	}

	projectRef, err := resolveProjectReference(ctx, scope)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	endpoint, err := buildServiceEndpoint(opts, projectRef)
	if err != nil {
		return util.FlagErrorf("failed to build service endpoint payload: %w", err)
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), scope.Organization)
	if err != nil {
		return err
	}

	createdEndpoint, err := client.CreateServiceEndpoint(ctx.Context(), serviceendpoint.CreateServiceEndpointArgs{
		Endpoint: endpoint,
	})
	if err != nil {
		return fmt.Errorf("failed to create service endpoint: %w", err)
	}

	zap.L().Debug("azurerm service endpoint created",
		zap.String("id", types.GetValue(createdEndpoint.Id, uuid.Nil).String()),
		zap.String("name", types.GetValue(createdEndpoint.Name, "")),
	)

	if opts.grantPermissionToAllPipelines {
		projectID := types.GetValue(projectRef.Id, uuid.Nil)
		if projectID == uuid.Nil {
			return errors.New("project reference missing ID")
		}

		endpointID := types.GetValue(createdEndpoint.Id, uuid.Nil)
		if endpointID == uuid.Nil {
			return errors.New("service endpoint create response missing ID")
		}

		if err := shared.GrantAllPipelinesAccessToEndpoint(ctx,
			scope.Organization,
			projectID,
			endpointID,
			func() error {
				return client.DeleteServiceEndpoint(ctx.Context(), serviceendpoint.DeleteServiceEndpointArgs{
					EndpointId: types.ToPtr(endpointID),
					ProjectIds: &[]string{projectID.String()},
				})
			}); err != nil {
			return err
		}

		zap.L().Debug("Granted all pipelines permission to service endpoint",
			zap.String("id", endpointID.String()),
		)
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, createdEndpoint)
	}

	tp, err := ctx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("ID", "Name", "Type", "URL")
	tp.EndRow()
	tp.AddField(types.GetValue(createdEndpoint.Id, uuid.Nil).String())
	tp.AddField(types.GetValue(createdEndpoint.Name, ""))
	tp.AddField(types.GetValue(createdEndpoint.Type, ""))
	tp.AddField(types.GetValue(createdEndpoint.Url, ""))
	tp.EndRow()
	tp.Render()

	return nil
}

func validateOpts(opts *createOptions, ios *iostreams.IOStreams, p prompter.Prompter) error {
	if opts.tenantID == "" {
		return errors.New("--tenant-id is required")
	}

	// Validate scope
	if opts.subscriptionID == "" && opts.managementGroupID == "" {
		return errors.New("one of --subscription-id or --management-group-id must be provided")
	}
	if opts.subscriptionID != "" && opts.managementGroupID != "" {
		return errors.New("--subscription-id and --management-group-id are mutually exclusive")
	}
	if opts.managementGroupID != "" && opts.managementGroupName == "" {
		return errors.New("--management-group-name is required when --management-group-id is specified")
	}
	if opts.subscriptionID != "" && opts.subscriptionName == "" {
		return errors.New("--subscription-name is required when --subscription-id is specified")
	}

	// Set creation mode
	hasCredentials := opts.servicePrincipalID != ""
	if opts.authenticationScheme == AuthSchemeServicePrincipal || opts.authenticationScheme == AuthSchemeWorkloadIdentityFederation {
		if hasCredentials {
			opts.serviceEndpointCreationMode = CreationModeManual
		} else {
			opts.serviceEndpointCreationMode = CreationModeAutomatic
		}
	}

	// Validate auth scheme specific requirements
	switch opts.authenticationScheme {
	case AuthSchemeServicePrincipal:
		if opts.serviceEndpointCreationMode == CreationModeAutomatic {
			return errors.New("automatic creation mode is not supported for ServicePrincipal from the CLI. Please provide --service-principal-id")
		}
		if opts.servicePrincipalKey == "" && opts.certificatePath == "" {
			if !ios.CanPrompt() {
				return errors.New("--service-principal-key not provided and prompting disabled")
			}
			secret, err := p.Password("Service principal key:")
			if err != nil {
				return fmt.Errorf("prompt for secret failed: %w", err)
			}
			opts.servicePrincipalKey = secret
		}
		if opts.servicePrincipalKey != "" && opts.certificatePath != "" {
			return errors.New("--service-principal-key and --certificate-path are mutually exclusive")
		}
		if opts.certificatePath != "" {
			certBytes, err := os.ReadFile(opts.certificatePath)
			if err != nil {
				return fmt.Errorf("failed to read certificate file: %w", err)
			}
			opts.servicePrincipalCertificate = string(certBytes)
		}
	case AuthSchemeWorkloadIdentityFederation:
		// This is a valid scenario, where ADO will configure the SPN.
	case AuthSchemeManagedServiceIdentity:
		// No specific validation needed
	default:
		return fmt.Errorf("invalid --authentication-scheme: %s", opts.authenticationScheme)
	}

	if opts.environment == "AzureStack" && opts.serverURL == "" {
		return errors.New("--server-url is required when environment is AzureStack")
	}

	return nil
}

func buildServiceEndpoint(opts *createOptions, projectRef *serviceendpoint.ProjectReference) (*serviceendpoint.ServiceEndpoint, error) {
	endpointType := "azurerm"
	endpointURL, err := getEndpointURL(opts)
	if err != nil {
		return nil, err
	}
	owner := "library"

	authParams := map[string]string{
		"tenantid": opts.tenantID,
	}

	data := map[string]string{
		"environment": opts.environment,
	}

	if opts.serviceEndpointCreationMode != "" && opts.authenticationScheme != AuthSchemeManagedServiceIdentity {
		data["creationMode"] = opts.serviceEndpointCreationMode
	}

	// Scope handling
	if opts.subscriptionID != "" {
		if opts.resourceGroup != "" && opts.authenticationScheme != AuthSchemeManagedServiceIdentity {
			authParams["scope"] = fmt.Sprintf("/subscriptions/%s/resourcegroups/%s", opts.subscriptionID, opts.resourceGroup)
		}
		data["scopeLevel"] = ScopeLevelSubscription
		data["subscriptionId"] = opts.subscriptionID
		data["subscriptionName"] = opts.subscriptionName

	} else if opts.managementGroupID != "" {
		data["scopeLevel"] = ScopeLevelManagementGroup
		data["managementGroupId"] = opts.managementGroupID
		data["managementGroupName"] = opts.managementGroupName
	}

	// Auth scheme specific logic
	switch opts.authenticationScheme {
	case AuthSchemeServicePrincipal:
		authParams["serviceprincipalid"] = opts.servicePrincipalID
		if opts.servicePrincipalKey != "" {
			authParams["authenticationType"] = "spnKey"
			authParams["serviceprincipalkey"] = opts.servicePrincipalKey
		} else if opts.servicePrincipalCertificate != "" {
			authParams["authenticationType"] = "spnCertificate"
			authParams["servicePrincipalCertificate"] = opts.servicePrincipalCertificate
		}
	case AuthSchemeWorkloadIdentityFederation:
		if opts.serviceEndpointCreationMode == CreationModeManual {
			if opts.servicePrincipalID == "" {
				return nil, errors.New("serviceprincipalid is required for WorkloadIdentityFederation in Manual mode")
			}
			authParams["serviceprincipalid"] = opts.servicePrincipalID
		} else {
			authParams["serviceprincipalid"] = ""
		}
	case AuthSchemeManagedServiceIdentity:
		// No extra auth params needed
	}

	return &serviceendpoint.ServiceEndpoint{
		Name:        &opts.name,
		Type:        &endpointType,
		Url:         &endpointURL,
		Description: &opts.description,
		Owner:       &owner,
		Authorization: &serviceendpoint.EndpointAuthorization{
			Scheme:     &opts.authenticationScheme,
			Parameters: &authParams,
		},
		Data: &data,
		ServiceEndpointProjectReferences: &[]serviceendpoint.ServiceEndpointProjectReference{
			{
				ProjectReference: projectRef,
				Name:             &opts.name,
				Description:      &opts.description,
			},
		},
	}, nil
}

func getEndpointURL(opts *createOptions) (string, error) {
	switch opts.environment {
	case "AzureCloud":
		return "https://management.azure.com/", nil
	case "AzureChinaCloud":
		return "https://management.chinacloudapi.cn/", nil
	case "AzureUSGovernment":
		return "https://management.usgovcloudapi.net/", nil
	case "AzureGermanCloud":
		return "https://management.microsoftazure.de/", nil
	case "AzureStack":
		return opts.serverURL, nil
	default:
		return "", fmt.Errorf("unknown environment: %s", opts.environment)
	}
}

func resolveProjectReference(ctx util.CmdContext, scope *util.Scope) (*serviceendpoint.ProjectReference, error) {
	coreClient, err := ctx.ClientFactory().Core(ctx.Context(), scope.Organization)
	if err != nil {
		return nil, fmt.Errorf("failed to create core client: %w", err)
	}

	project, err := coreClient.GetProject(ctx.Context(), core.GetProjectArgs{
		ProjectId: types.ToPtr(scope.Project),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project %q: %w", scope.Project, err)
	}
	if project == nil || project.Id == nil {
		return nil, fmt.Errorf("project %q does not expose an ID", scope.Project)
	}

	return &serviceendpoint.ProjectReference{
		Id:   project.Id,
		Name: project.Name,
	}, nil
}
