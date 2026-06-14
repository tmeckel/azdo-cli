package shared

import (
	_ "embed"
	"fmt"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
)

//go:embed show.tpl
var showTpl string

// Output renders the service endpoint details to the output stream.
// If an exporter is provided, it writes the endpoint using the exporter.
// Otherwise, it uses the shared template to render the endpoint details.
func Output(ctx util.CmdContext, endpoint *serviceendpoint.ServiceEndpoint, exporter util.Exporter) error {
	ios, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	if exporter != nil {
		return exporter.Write(ios, endpoint)
	}

	t := template.New(
		ios.Out,
		ios.TerminalWidth(),
		ios.ColorEnabled(),
	).
		WithTheme(ios.TerminalTheme()).
		WithFuncs(map[string]any{
			"s":       template.StringOrEmpty,
			"hasText": template.HasText,
			"b":       template.BoolString,
			"u":       template.UUIDString,
			"scheme": func(ep *serviceendpoint.EndpointAuthorization) string {
				// We wrap shared.AuthorizationScheme to work with just the authorization part if needed
				// or we can just pass the whole endpoint.
				// Since we already have shared.AuthorizationScheme(ep *serviceendpoint.ServiceEndpoint)
				// let's define a helper that takes just the authorization.
				if ep == nil || ep.Scheme == nil {
					return ""
				}
				return *ep.Scheme
			},
			"identity": func(id *webapi.IdentityRef) string {
				if id == nil || id.DisplayName == nil {
					return ""
				}
				return fmt.Sprintf("%s (%s)", *id.DisplayName, *id.UniqueName)
			},
		})
	err = t.Parse(showTpl)
	if err != nil {
		return err
	}

	return t.ExecuteData(endpoint)
}
