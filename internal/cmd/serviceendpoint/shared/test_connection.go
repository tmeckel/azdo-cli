package shared

import (
	"fmt"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/serviceendpoint"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	pollutil "github.com/tmeckel/azdo-cli/internal/util"
)

// TestConnection executes the endpoint type's TestConnection data source when available.
// It polls until the data source reports StatusCode == "ok" or timeout/context cancel.
func TestConnection(cmdCtx util.CmdContext, client serviceendpoint.Client, organization, project string, ep *serviceendpoint.ServiceEndpoint, timeout time.Duration) error {
	if ep == nil {
		return fmt.Errorf("endpoint required")
	}
	if ep.Type == nil {
		return fmt.Errorf("endpoint.Type required")
	}

	types, err := GetServiceEndpointTypes(cmdCtx, organization)
	if err != nil {
		return err
	}

	var matched *serviceendpoint.ServiceEndpointType
	for _, t := range types {
		if t.Name != nil && *t.Name == *ep.Type {
			matched = &t
			break
		}
	}
	if matched == nil {
		return fmt.Errorf("unknown service endpoint type: %s", *ep.Type)
	}

	// Find TestConnection data source
	var ds *serviceendpoint.DataSource
	if matched.DataSources != nil {
		for _, d := range *matched.DataSources {
			if d.Name != nil && strings.EqualFold(*d.Name, "TestConnection") {
				ds = &d
				break
			}
		}
	}
	if ds == nil {
		return fmt.Errorf("TestConnection not supported for endpoint type %s", *ep.Type)
	}

	ctx := cmdCtx.Context()
	opts := pollutil.PollOptions{}
	if timeout > 0 {
		opts.Timeout = timeout
	}

	// Build request template
	name := "TestConnection"
	req := &serviceendpoint.ServiceEndpointRequest{
		DataSourceDetails: &serviceendpoint.DataSourceDetails{DataSourceName: &name},
		ServiceEndpointDetails: &serviceendpoint.ServiceEndpointDetails{
			Data:          ep.Data,
			Authorization: ep.Authorization,
			Url:           ep.Url,
			Type:          ep.Type,
		},
		ResultTransformationDetails: &serviceendpoint.ResultTransformationDetails{},
	}

	var lastErr error
	err = pollutil.Poll(ctx, func() error {
		res, err := client.ExecuteServiceEndpointRequest(ctx, serviceendpoint.ExecuteServiceEndpointRequestArgs{ServiceEndpointRequest: req})
		if err != nil {
			lastErr = err
			return err
		}
		if res == nil || res.StatusCode == nil {
			lastErr = fmt.Errorf("test connection returned empty result")
			return lastErr
		}
		if strings.EqualFold(*res.StatusCode, "ok") {
			return nil
		}
		// not ok => retry until timeout
		lastErr = fmt.Errorf("test connection status: %s", *res.StatusCode)
		return lastErr
	}, opts)
	if err != nil {
		if lastErr != nil {
			return fmt.Errorf("test connection failed: %w", lastErr)
		}
		return err
	}
	return nil
}
