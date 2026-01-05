package azurerm

import (
	"errors"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/spf13/cobra"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
)

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

type createOptions struct {
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
}

type azurermConfigurer struct {
	cmdCtx util.CmdContext
	createOptions
}

func (g *azurermConfigurer) CommandContext() util.CmdContext {
	return g.cmdCtx
}

func (c *azurermConfigurer) TypeName() string {
	return "azurerm"
}

func (c *azurermConfigurer) Configure(endpoint *serviceendpoint.ServiceEndpoint) error {
	err := c.validateOpts()
	if err != nil {
		return err
	}

	url, err := c.getEndpointURL()
	if err != nil {
		return err
	}

	endpoint.Url = &url

	authParams := map[string]string{
		"tenantid": c.tenantID,
	}

	data := map[string]string{
		"environment": c.environment,
	}

	if c.serviceEndpointCreationMode != "" && c.authenticationScheme != AuthSchemeManagedServiceIdentity {
		data["creationMode"] = c.serviceEndpointCreationMode
	}

	// Scope handling
	if c.subscriptionID != "" {
		if c.resourceGroup != "" && c.authenticationScheme != AuthSchemeManagedServiceIdentity {
			authParams["scope"] = fmt.Sprintf("/subscriptions/%s/resourcegroups/%s", c.subscriptionID, c.resourceGroup)
		}
		data["scopeLevel"] = ScopeLevelSubscription
		data["subscriptionId"] = c.subscriptionID
		data["subscriptionName"] = c.subscriptionName

	} else if c.managementGroupID != "" {
		data["scopeLevel"] = ScopeLevelManagementGroup
		data["managementGroupId"] = c.managementGroupID
		data["managementGroupName"] = c.managementGroupName
	}

	// Auth scheme specific logic
	switch c.authenticationScheme {
	case AuthSchemeServicePrincipal:
		authParams["serviceprincipalid"] = c.servicePrincipalID
		if c.servicePrincipalKey != "" {
			authParams["authenticationType"] = "spnKey"
			authParams["serviceprincipalkey"] = c.servicePrincipalKey
		} else if c.servicePrincipalCertificate != "" {
			authParams["authenticationType"] = "spnCertificate"
			authParams["servicePrincipalCertificate"] = c.servicePrincipalCertificate
		}
	case AuthSchemeWorkloadIdentityFederation:
		if c.serviceEndpointCreationMode == CreationModeManual {
			if c.servicePrincipalID == "" {
				return errors.New("serviceprincipalid is required for WorkloadIdentityFederation in Manual mode")
			}
			authParams["serviceprincipalid"] = c.servicePrincipalID
		} else {
			authParams["serviceprincipalid"] = ""
		}
	case AuthSchemeManagedServiceIdentity:
		// No extra auth params needed
	}

	endpoint.Authorization = &serviceendpoint.EndpointAuthorization{
		Scheme:     &c.authenticationScheme,
		Parameters: &authParams,
	}
	endpoint.Data = &data
	return nil
}

func (c *azurermConfigurer) validateOpts() error {
	if c.tenantID == "" {
		return errors.New("--tenant-id is required")
	}

	// Validate scope
	if c.subscriptionID == "" && c.managementGroupID == "" {
		return errors.New("one of --subscription-id or --management-group-id must be provided")
	}
	if c.subscriptionID != "" && c.managementGroupID != "" {
		return errors.New("--subscription-id and --management-group-id are mutually exclusive")
	}
	if c.managementGroupID != "" && c.managementGroupName == "" {
		return errors.New("--management-group-name is required when --management-group-id is specified")
	}
	if c.subscriptionID != "" && c.subscriptionName == "" {
		return errors.New("--subscription-name is required when --subscription-id is specified")
	}

	// Set creation mode
	hasCredentials := c.servicePrincipalID != ""
	if c.authenticationScheme == AuthSchemeServicePrincipal || c.authenticationScheme == AuthSchemeWorkloadIdentityFederation {
		if hasCredentials {
			c.serviceEndpointCreationMode = CreationModeManual
		} else {
			c.serviceEndpointCreationMode = CreationModeAutomatic
		}
	}

	// Validate auth scheme specific requirements
	switch c.authenticationScheme {
	case AuthSchemeServicePrincipal:
		if c.serviceEndpointCreationMode == CreationModeAutomatic {
			return errors.New("automatic creation mode is not supported for ServicePrincipal from the CLI. Please provide --service-principal-id")
		}
		if c.servicePrincipalKey == "" && c.certificatePath == "" {
			cmdCtx := c.cmdCtx
			ios, err := cmdCtx.IOStreams()
			if err != nil {
				return err
			}

			if !ios.CanPrompt() {
				return errors.New("--service-principal-key not provided and prompting disabled")
			}

			p, err := cmdCtx.Prompter()
			if err != nil {
				return err
			}

			secret, err := p.Password("Service principal key:")
			if err != nil {
				return fmt.Errorf("prompt for secret failed: %w", err)
			}
			c.servicePrincipalKey = secret
		}
		if c.servicePrincipalKey != "" && c.certificatePath != "" {
			return errors.New("--service-principal-key and --certificate-path are mutually exclusive")
		}
		if c.certificatePath != "" {
			certBytes, err := os.ReadFile(c.certificatePath)
			if err != nil {
				return fmt.Errorf("failed to read certificate file: %w", err)
			}
			c.servicePrincipalCertificate = string(certBytes)
		}
	case AuthSchemeWorkloadIdentityFederation:
		// This is a valid scenario, where ADO will configure the SPN.
	case AuthSchemeManagedServiceIdentity:
		// No specific validation needed
	default:
		return fmt.Errorf("invalid --authentication-scheme: %s", c.authenticationScheme)
	}

	if c.environment == "AzureStack" && c.serverURL == "" {
		return errors.New("--server-url is required when environment is AzureStack")
	}

	return nil
}

func (c *azurermConfigurer) getEndpointURL() (string, error) {
	switch c.environment {
	case "AzureCloud":
		return "https://management.azure.com/", nil
	case "AzureChinaCloud":
		return "https://management.chinacloudapi.cn/", nil
	case "AzureUSGovernment":
		return "https://management.usgovcloudapi.net/", nil
	case "AzureGermanCloud":
		return "https://management.microsoftazure.de/", nil
	case "AzureStack":
		return c.serverURL, nil
	default:
		return "", fmt.Errorf("unknown environment: %s", c.environment)
	}
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	cfg := &azurermConfigurer{
		cmdCtx: ctx,
	}

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
			return shared.RunTypedCreate(cmd, args, cfg)
		},
	}

	util.StringEnumFlag(cmd, &cfg.authenticationScheme, "authentication-scheme", "", AuthSchemeServicePrincipal,
		[]string{AuthSchemeServicePrincipal, AuthSchemeManagedServiceIdentity, AuthSchemeWorkloadIdentityFederation},
		"Authentication scheme")
	cmd.Flags().StringVar(&cfg.tenantID, "tenant-id", "", "Azure tenant ID (e.g., GUID)")
	cmd.Flags().StringVar(&cfg.subscriptionID, "subscription-id", "", "Azure subscription ID (e.g., GUID)")
	cmd.Flags().StringVar(&cfg.subscriptionName, "subscription-name", "", "Azure subscription name")
	cmd.Flags().StringVar(&cfg.managementGroupID, "management-group-id", "", "Azure management group ID")
	cmd.Flags().StringVar(&cfg.managementGroupName, "management-group-name", "", "Azure management group name")
	cmd.Flags().StringVar(&cfg.resourceGroup, "resource-group", "", "Name of the resource group (for subscription-level scope)")
	cmd.Flags().StringVar(&cfg.servicePrincipalID, "service-principal-id", "", "Service principal/application ID (e.g., GUID)")
	cmd.Flags().StringVar(&cfg.servicePrincipalKey, "service-principal-key", "", "Service principal key (secret value)")
	cmd.Flags().StringVar(&cfg.certificatePath, "certificate-path", "", "Path to service principal certificate file (PEM format)")
	util.StringEnumFlag(cmd, &cfg.environment, "environment", "", "AzureCloud",
		[]string{"AzureCloud", "AzureChinaCloud", "AzureUSGovernment", "AzureGermanCloud", "AzureStack"},
		"Azure environment")
	cmd.Flags().StringVar(&cfg.serverURL, "server-url", "", "Azure Stack Resource Manager base URL. Required if --environment is AzureStack.")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("authentication-scheme")

	return shared.AddCreateCommonFlags(cmd)
}
