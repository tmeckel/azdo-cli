package create

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelinepermissions"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/types"
	"go.uber.org/zap"
)

const (
	defaultGroupType      = "Vsts"
	keyVaultGroupType     = "AzureKeyVault"
	secretEnvPrefix       = "AZDO_PIPELINES_SECRET_"
	variableGroupResource = "variablegroup"
	readOnlyToken         = "readonly"
	keyValueSeparator     = "="
)

type options struct {
	targetArg   string
	description string
	groupType   string
	authorize   bool

	projectReferences []string
	variableInputs    []string
	secretInputs      []string

	keyVaultServiceEndpoint string
	keyVaultName            string
	keyVaultSecrets         []string

	providerDataJSON string

	exporter util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &options{
		groupType: defaultGroupType,
	}

	cmd := &cobra.Command{
		Use:   "create [ORGANIZATION/]PROJECT/NAME",
		Short: "Create a variable group",
		Long: heredoc.Doc(`
			Create a variable group in a project, optionally seeding variables, linking Key Vault secrets,
			authorizing access for all pipelines, and sharing the group across additional projects.
		`),
		Example: heredoc.Doc(`
			# Create a generic variable group with seeded variables
			azdo pipelines variable-group create MyOrg/MyProject/SharedConfig \
				--description "Shared non-secret settings" \
				--variable env=prod --variable region=westus;readOnly=true

			# Create a Key Vault-backed variable group authorized for all pipelines
			azdo pipelines variable-group create MyProject/SensitiveSecrets \
				--keyvault-service-endpoint 11111111-2222-3333-4444-555555555555 \
				--keyvault-name my-vault --keyvault-secret dbPassword=ProdDbSecret \
				--authorize

			# Share a group with an additional project by name
			azdo pipelines variable-group create MyOrg/ProjectA/CrossProject \
				--project-reference ProjectB
		`),
		Args: util.ExactArgs(1, "target argument is required and must be in the form [ORGANIZATION/]PROJECT/NAME"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return run(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.description, "description", "d", "", "Optional description for the variable group")
	util.StringEnumFlag(cmd, &opts.groupType, "type", "", defaultGroupType, []string{defaultGroupType, keyVaultGroupType}, "Variable group type")
	cmd.Flags().BoolVarP(&opts.authorize, "authorize", "A", false, "Authorize the variable group for all pipelines in the project after creation")
	cmd.Flags().StringSliceVar(&opts.projectReferences, "project-reference", nil, "Additional project names or IDs to share the group with (repeat or comma-separate)")
	cmd.Flags().StringSliceVarP(&opts.variableInputs, "variable", "v", nil, "Seed non-secret variables using key=value[;readOnly=true|false]")
	cmd.Flags().StringSliceVar(&opts.secretInputs, "secret", nil, "Seed secret variables using key[=value]; value falls back to AZDO_PIPELINES_SECRET_<NAME> or an interactive prompt")
	cmd.Flags().StringVar(&opts.keyVaultServiceEndpoint, "keyvault-service-endpoint", "", "Service endpoint ID (UUID) or name, that grants access to the Azure Key Vault")
	cmd.Flags().StringVar(&opts.keyVaultName, "keyvault-name", "", "Azure Key Vault name backing the variable group")
	cmd.Flags().StringSliceVar(&opts.keyVaultSecrets, "keyvault-secret", nil, "Map a pipeline variable to a Key Vault secret (variable=secretName); repeat for multiple entries")
	cmd.Flags().StringVar(&opts.providerDataJSON, "provider-data-json", "", "Raw JSON payload for providerData (advanced; cannot be combined with Key Vault options)")

	util.AddJSONFlags(cmd, &opts.exporter, []string{
		"createdBy",
		"createdOn",
		"description",
		"id",
		"isShared",
		"modifiedBy",
		"modifiedOn",
		"name",
		"providerData",
		"type",
		"variableGroupProjectReferences",
		"variables",
	})

	return cmd
}

func run(cmdCtx util.CmdContext, opts *options) error {
	ios, err := cmdCtx.IOStreams()
	if err != nil {
		return err
	}
	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	target, err := util.ParseProjectTargetWithDefaultOrganization(cmdCtx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	groupName := strings.TrimSpace(target.Target)
	if groupName == "" {
		return util.FlagErrorf("variable group name cannot be empty")
	}

	keyVaultRequested := opts.keyVaultServiceEndpoint != "" || opts.keyVaultName != "" || len(opts.keyVaultSecrets) > 0 || strings.EqualFold(opts.groupType, keyVaultGroupType)
	if len(opts.variableInputs) > 0 && keyVaultRequested {
		return util.FlagErrorf("--variable cannot be used with Key Vault options")
	}
	if len(opts.secretInputs) > 0 && keyVaultRequested {
		return util.FlagErrorf("--secret cannot be used with Key Vault options")
	}
	if opts.providerDataJSON != "" && keyVaultRequested {
		return util.FlagErrorf("--provider-data-json cannot be combined with Key Vault options")
	}

	coreClient, err := cmdCtx.ClientFactory().Core(cmdCtx.Context(), target.Organization)
	if err != nil {
		return err
	}

	projectRef, err := coreClient.GetProject(cmdCtx.Context(), core.GetProjectArgs{
		ProjectId: types.ToPtr(target.Project),
	})
	if err != nil {
		return fmt.Errorf("failed to resolve project %q: %w", target.Project, err)
	}
	if projectRef == nil || projectRef.Id == nil {
		return fmt.Errorf("project %q is missing an ID", target.Project)
	}

	projectName := types.GetValue(projectRef.Name, target.Project)

	logger := zap.L().With(
		zap.String("organization", target.Organization),
		zap.String("project", projectName),
		zap.String("group", groupName),
	)
	logger.Debug("creating variable group")

	var providerData interface{}
	if keyVaultRequested {
		providerData, err = buildKeyVaultProviderData(cmdCtx, target.Scope, opts)
		if err != nil {
			return util.FlagErrorWrap(err)
		}
		opts.groupType = keyVaultGroupType
	} else if strings.TrimSpace(opts.providerDataJSON) != "" {
		providerData, err = parseProviderDataJSON(opts.providerDataJSON)
		if err != nil {
			return util.FlagErrorWrap(err)
		}
	}

	variables, err := buildVariables(cmdCtx, ios, opts, keyVaultRequested)
	if err != nil {
		return err
	}

	params := taskagent.VariableGroupParameters{
		Name:         types.ToPtr(groupName),
		ProviderData: providerData,
		Type:         types.ToPtr(opts.groupType),
	}
	if trimmed := strings.TrimSpace(opts.description); trimmed != "" {
		params.Description = &opts.description
	}
	if variables != nil {
		params.Variables = variables
	}

	projectReferences, err := buildProjectReferences(cmdCtx, coreClient, projectRef, groupName, opts.projectReferences)
	if err != nil {
		return err
	}
	if len(projectReferences) > 0 {
		params.VariableGroupProjectReferences = &projectReferences
	}

	client, err := cmdCtx.ClientFactory().TaskAgent(cmdCtx.Context(), target.Organization)
	if err != nil {
		return err
	}

	created, err := client.AddVariableGroup(cmdCtx.Context(), taskagent.AddVariableGroupArgs{
		VariableGroupParameters: &params,
	})
	if err != nil {
		return fmt.Errorf("failed to create variable group: %w", err)
	}

	if opts.authorize {
		if err := authorizeVariableGroup(cmdCtx, target.Organization, target.Project, created); err != nil {
			return err
		}
	}

	ios.StopProgressIndicator()

	if opts.exporter != nil {
		return opts.exporter.Write(ios, created)
	}

	tp, err := cmdCtx.Printer("list")
	if err != nil {
		return err
	}
	tp.AddColumns("ID", "NAME", "TYPE", "DESCRIPTION", "VARIABLE COUNT")
	tp.EndRow()
	variableCount := 0
	if created != nil && created.Variables != nil {
		variableCount = len(*created.Variables)
	}
	tp.AddField(strconv.Itoa(types.GetValue(created.Id, 0)))
	tp.AddField(types.GetValue(created.Name, ""))
	tp.AddField(types.GetValue(created.Type, ""))
	tp.AddField(types.GetValue(created.Description, ""))
	tp.AddField(strconv.Itoa(variableCount))
	tp.EndRow()
	return tp.Render()
}

func buildVariables(cmdCtx util.CmdContext, ios *iostreams.IOStreams, opts *options, keyVault bool) (*map[string]interface{}, error) {
	if keyVault {
		return buildKeyVaultVariables(opts)
	}
	if len(opts.variableInputs) == 0 && len(opts.secretInputs) == 0 {
		return nil, nil
	}
	variables := make(map[string]interface{})
	seen := make(map[string]string)

	for _, raw := range opts.variableInputs {
		entry, err := parseVariableInput(raw)
		if err != nil {
			return nil, err
		}
		keyLower := strings.ToLower(entry.Name)
		if prev, exists := seen[keyLower]; exists {
			return nil, util.FlagErrorf("duplicate variable %q conflicts with %q", entry.Name, prev)
		}
		seen[keyLower] = entry.Name
		variables[entry.Name] = taskagent.VariableValue{
			Value:      types.ToPtr(entry.Value),
			IsSecret:   types.ToPtr(false),
			IsReadOnly: entry.ReadOnly,
		}
	}

	for _, raw := range opts.secretInputs {
		name, explicit, err := parseSecretInput(raw)
		if err != nil {
			return nil, err
		}
		keyLower := strings.ToLower(name)
		if prev, exists := seen[keyLower]; exists {
			return nil, util.FlagErrorf("duplicate variable %q conflicts with %q", name, prev)
		}
		value, err := resolveSecretValue(cmdCtx, ios, name, explicit)
		if err != nil {
			return nil, err
		}
		seen[keyLower] = name
		variables[name] = taskagent.VariableValue{
			Value:    types.ToPtr(value),
			IsSecret: types.ToPtr(true),
		}
	}

	return &variables, nil
}

func buildKeyVaultVariables(opts *options) (*map[string]interface{}, error) {
	if len(opts.keyVaultSecrets) == 0 {
		return nil, util.FlagErrorf("at least one --keyvault-secret is required when configuring a Key Vault-backed group")
	}
	variables := make(map[string]interface{}, len(opts.keyVaultSecrets))
	seen := make(map[string]struct{})
	for _, raw := range opts.keyVaultSecrets {
		parts := strings.SplitN(raw, keyValueSeparator, 2)
		if len(parts) != 2 {
			return nil, util.FlagErrorf("invalid --keyvault-secret %q; expected format variable=secretName", raw)
		}
		name := strings.TrimSpace(parts[0])
		secretName := strings.TrimSpace(parts[1])
		if name == "" || secretName == "" {
			return nil, util.FlagErrorf("invalid --keyvault-secret %q; names must not be empty", raw)
		}
		lower := strings.ToLower(name)
		if _, exists := seen[lower]; exists {
			return nil, util.FlagErrorf("duplicate Key Vault variable %q", name)
		}
		seen[lower] = struct{}{}
		variables[name] = taskagent.VariableValue{
			Value:    types.ToPtr(secretName),
			IsSecret: types.ToPtr(true),
		}
	}
	return &variables, nil
}

func buildKeyVaultProviderData(
	cmdCtx util.CmdContext,
	scope util.Scope,
	opts *options,
) (*taskagent.AzureKeyVaultVariableGroupProviderData, error) {
	if opts.keyVaultServiceEndpoint == "" {
		return nil, util.FlagErrorf("--keyvault-service-endpoint is required for Key Vault-backed groups")
	}
	if opts.keyVaultName == "" {
		return nil, util.FlagErrorf("--keyvault-name is required for Key Vault-backed groups")
	}
	serviceEndpointClient, err := cmdCtx.ClientFactory().ServiceEndpoint(cmdCtx.Context(), scope.Organization)
	if err != nil {
		return nil, err
	}
	identifier := strings.TrimSpace(opts.keyVaultServiceEndpoint)
	endpoint, err := shared.FindServiceEndpoint(cmdCtx, serviceEndpointClient, scope.Project, identifier)
	if err != nil {
		if errors.Is(err, shared.ErrEndpointNotFound) {
			return nil, util.FlagErrorf("service endpoint %q not found in project %s", identifier, scope.Project)
		}
		return nil, err
	}
	if endpoint == nil || endpoint.Id == nil {
		return nil, fmt.Errorf("service endpoint %q returned without an ID", identifier)
	}
	return &taskagent.AzureKeyVaultVariableGroupProviderData{
		ServiceEndpointId: endpoint.Id,
		Vault:             types.ToPtr(strings.TrimSpace(opts.keyVaultName)),
	}, nil
}

func buildProjectReferences(
	cmdCtx util.CmdContext,
	coreClient core.Client,
	primary *core.TeamProject,
	groupName string,
	additional []string,
) ([]taskagent.VariableGroupProjectReference, error) {
	refs := []taskagent.VariableGroupProjectReference{
		{
			Name: types.ToPtr(groupName),
			ProjectReference: &taskagent.ProjectReference{
				Id:   primary.Id,
				Name: primary.Name,
			},
		},
	}

	if len(additional) == 0 {
		return refs, nil
	}

	seen := map[string]struct{}{primary.Id.String(): {}}
	for _, raw := range additional {
		for _, token := range splitAndTrim(raw) {
			if token == "" {
				return nil, util.FlagErrorf("project reference values must not be empty")
			}
			projRef, err := resolveProjectReference(cmdCtx, coreClient, token)
			if err != nil {
				return nil, err
			}
			id := projRef.Id.String()
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			refs = append(refs, taskagent.VariableGroupProjectReference{
				Name: types.ToPtr(groupName),
				ProjectReference: &taskagent.ProjectReference{
					Id:   projRef.Id,
					Name: projRef.Name,
				},
			})
		}
	}
	return refs, nil
}

func resolveProjectReference(
	cmdCtx util.CmdContext,
	client core.Client,
	value string,
) (*core.TeamProjectReference, error) {
	if id, err := uuid.Parse(value); err == nil {
		return &core.TeamProjectReference{Id: &id}, nil
	}
	proj, err := client.GetProject(cmdCtx.Context(), core.GetProjectArgs{ProjectId: types.ToPtr(value)})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project reference %q: %w", value, err)
	}
	if proj == nil || proj.Id == nil {
		return nil, fmt.Errorf("project reference %q is missing an ID", value)
	}
	return &core.TeamProjectReference{
		Id:   proj.Id,
		Name: proj.Name,
	}, nil
}

func authorizeVariableGroup(
	cmdCtx util.CmdContext,
	organization string,
	project string,
	group *taskagent.VariableGroup,
) error {
	if group == nil || group.Id == nil {
		return errors.New("variable group ID is missing; cannot authorize")
	}
	permissionsClient, err := cmdCtx.ClientFactory().PipelinePermissions(cmdCtx.Context(), organization)
	if err != nil {
		return err
	}
	projectArg := project
	resourceType := variableGroupResource
	resourceID := strconv.Itoa(types.GetValue(group.Id, 0))
	authorized := true
	_, err = permissionsClient.UpdatePipelinePermisionsForResource(cmdCtx.Context(), pipelinepermissions.UpdatePipelinePermisionsForResourceArgs{
		Project:      &projectArg,
		ResourceType: &resourceType,
		ResourceId:   &resourceID,
		ResourceAuthorization: &pipelinepermissions.ResourcePipelinePermissions{
			AllPipelines: &pipelinepermissions.Permission{
				Authorized: &authorized,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to authorize variable group for pipelines: %w", err)
	}
	return nil
}

func parseVariableInput(raw string) (*variableInput, error) {
	parts := strings.SplitN(raw, keyValueSeparator, 2)
	if len(parts) != 2 {
		return nil, util.FlagErrorf("invalid --variable %q; expected format key=value[;readOnly=true|false]", raw)
	}
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return nil, util.FlagErrorf("invalid --variable %q; key cannot be empty", raw)
	}
	valueSegment := parts[1]
	segments := strings.Split(valueSegment, ";")
	value := segments[0]
	var readOnly *bool
	if len(segments) > 1 {
		for _, segment := range segments[1:] {
			kv := strings.SplitN(segment, "=", 2)
			if len(kv) != 2 {
				return nil, util.FlagErrorf("invalid --variable option %q", raw)
			}
			if strings.EqualFold(strings.TrimSpace(kv[0]), readOnlyToken) {
				parsed, err := strconv.ParseBool(strings.TrimSpace(kv[1]))
				if err != nil {
					return nil, util.FlagErrorf("invalid readOnly value in --variable %q: %v", raw, err)
				}
				readOnly = types.ToPtr(parsed)
			} else {
				return nil, util.FlagErrorf("unsupported option %q in --variable", kv[0])
			}
		}
	}
	return &variableInput{Name: name, Value: value, ReadOnly: readOnly}, nil
}

type variableInput struct {
	Name     string
	Value    string
	ReadOnly *bool
}

func parseSecretInput(raw string) (string, string, error) {
	parts := strings.SplitN(raw, keyValueSeparator, 2)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", "", util.FlagErrorf("invalid --secret %q; name cannot be empty", raw)
	}
	if len(parts) == 1 {
		return name, "", nil
	}
	return name, parts[1], nil
}

func resolveSecretValue(
	cmdCtx util.CmdContext,
	ios *iostreams.IOStreams,
	name string,
	explicit string,
) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	envKey := strings.ToUpper(secretEnvPrefix + "_" + name)
	if value, ok := os.LookupEnv(envKey); ok {
		return value, nil
	}
	if !ios.CanPrompt() {
		return "", fmt.Errorf("no value provided for secret %q and prompting is disabled", name)
	}
	ios.StopProgressIndicator()
	defer ios.StartProgressIndicator()
	prompter, err := cmdCtx.Prompter()
	if err != nil {
		return "", err
	}
	return prompter.Secret(fmt.Sprintf("Value for secret %q:", name))
}

func parseProviderDataJSON(raw string) (interface{}, error) {
	var payload interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, util.FlagErrorf("invalid --provider-data-json: %v", err)
	}
	return payload, nil
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
