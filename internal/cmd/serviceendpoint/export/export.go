package export

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/tmeckel/azdo-cli/internal/cmd/serviceendpoint/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

const secretPlaceholder = "__SECRET__"

type exportOptions struct {
	targetArg   string
	outputFile  string
	withSecrets bool
}

type exportedServiceEndpoint struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	URL           string                 `json:"url"`
	Description   *string                `json:"description,omitempty"`
	IsShared      *bool                  `json:"isShared,omitempty"`
	Authorization *exportedAuthorization `json:"authorization,omitempty"`
	Data          map[string]string      `json:"data,omitempty"`
}

type exportedAuthorization struct {
	Scheme     *string           `json:"scheme,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &exportOptions{}

	cmd := &cobra.Command{
		Use:   "export [ORGANIZATION/]PROJECT/ID_OR_NAME",
		Short: "Export a service endpoint definition as JSON.",
		Long: heredoc.Doc(`
			Export an Azure DevOps service endpoint (service connection) into a JSON definition.

			The positional argument accepts the form [ORGANIZATION/]PROJECT/ID_OR_NAME. When the
			organization segment is omitted the default organization from configuration is used.`),
		Example: heredoc.Doc(`
			# Export to stdout with secrets redacted
			azdo service-endpoint export my-org/MyProject/MyEndpoint

			# Export to a file while including secrets
			azdo service-endpoint export MyProject/058bff6f-2717-4500-af7e-3fffc2b0b546 --output-file ./endpoint.json --with-secrets
		`),
		Aliases: []string{"e", "ex"},
		Args:    util.ExactArgs(1, "service endpoint target required"),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.targetArg = args[0]
			return runExport(ctx, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.outputFile, "output-file", "o", "", "Path to write the exported JSON. Defaults to stdout.")
	cmd.Flags().BoolVar(&opts.withSecrets, "with-secrets", false, "Include sensitive authorization values in the export.")

	return cmd
}

func runExport(ctx util.CmdContext, opts *exportOptions) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	ios.StartProgressIndicator()
	defer ios.StopProgressIndicator()

	scope, err := util.ParseProjectTargetWithDefaultOrganization(ctx, opts.targetArg)
	if err != nil {
		return util.FlagErrorWrap(err)
	}

	client, err := ctx.ClientFactory().ServiceEndpoint(ctx.Context(), scope.Organization)
	if err != nil {
		return fmt.Errorf("failed to create service endpoint client: %w", err)
	}

	endpoint, err := shared.FindServiceEndpoint(ctx, client, scope.Project, scope.Target)
	if err != nil {
		if errors.Is(err, shared.ErrEndpointNotFound) {
			return fmt.Errorf("service endpoint %q was not found in %s/%s", scope.Target, scope.Organization, scope.Project)
		}
		return err
	}

	if endpoint == nil {
		return fmt.Errorf("service endpoint %q could not be resolved", scope.Target)
	}

	payload := &exportedServiceEndpoint{
		Name:     strings.TrimSpace(types.GetValue(endpoint.Name, scope.Target)),
		Type:     strings.TrimSpace(types.GetValue(endpoint.Type, "")),
		URL:      strings.TrimSpace(types.GetValue(endpoint.Url, "")),
		IsShared: endpoint.IsShared,
	}

	if desc := strings.TrimSpace(types.GetValue(endpoint.Description, "")); desc != "" {
		payload.Description = types.ToPtr(desc)
	}

	redacted := false
	if endpoint.Authorization != nil {
		var params map[string]string
		if endpoint.Authorization.Parameters != nil {
			params = make(map[string]string, len(*endpoint.Authorization.Parameters))
			for key, value := range *endpoint.Authorization.Parameters {
				if opts.withSecrets {
					params[key] = value
					continue
				}
				params[key] = secretPlaceholder
				if value != secretPlaceholder {
					redacted = true
				}
			}
		}
		payload.Authorization = &exportedAuthorization{
			Scheme:     endpoint.Authorization.Scheme,
			Parameters: params,
		}
	}

	if endpoint.Data != nil && len(*endpoint.Data) > 0 {
		payload.Data = make(map[string]string, len(*endpoint.Data))
		for key, value := range *endpoint.Data {
			payload.Data[key] = value
		}
	}

	zap.L().Debug("Exporting service endpoint",
		zap.String("organization", scope.Organization),
		zap.String("project", scope.Project),
		zap.String("identifier", scope.Target),
		zap.Bool("withSecrets", opts.withSecrets),
		zap.String("destination", func(path string) string {
			if strings.TrimSpace(path) == "" {
				return "stdout"
			}
			return path
		}(opts.outputFile)),
	)

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode service endpoint JSON: %w", err)
	}

	ios.StopProgressIndicator()

	if strings.TrimSpace(opts.outputFile) == "" {
		fmt.Fprintln(ios.Out, string(data))
	} else {
		fileBytes := append(append([]byte(nil), data...), '\n')
		if err := os.WriteFile(opts.outputFile, fileBytes, 0o600); err != nil {
			return fmt.Errorf("failed to write %s: %w", opts.outputFile, err)
		}

		cs := ios.ColorScheme()
		fmt.Fprintf(ios.Out, "%s Export wrote %s (%d bytes).\n", cs.SuccessIcon(), opts.outputFile, len(fileBytes))
	}

	if redacted {
		cs := ios.ColorScheme()
		fmt.Fprintf(ios.ErrOut, "%s Sensitive authorization values replaced with %q. Update them before reusing this file.\n", cs.WarningIcon(), secretPlaceholder)
	} else if opts.withSecrets {
		cs := ios.ColorScheme()
		fmt.Fprintf(ios.ErrOut, "%s Export includes live secrets. Store the file securely.\n", cs.WarningIcon())
	}

	return nil
}
