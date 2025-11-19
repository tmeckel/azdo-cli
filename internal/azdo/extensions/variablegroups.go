package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/taskagent"
)

// VariableGroupsResponse represents the REST payload returned by
// GET https://dev.azure.com/{organization}/{project}/_apis/distributedtask/variablegroups
type VariableGroupsResponse struct {
	Count             *int                      `json:"count,omitempty"`
	ContinuationToken *string                   `json:"continuationToken,omitempty"`
	Value             []taskagent.VariableGroup `json:"value,omitempty"`
}

type VariableValue struct {
	Name     string `json:"name"`
	Value    any    `json:"value,omitempty"`
	IsSecret bool   `json:"secret"`
}

// ToVariableValues converts a map of raw variable data into a slice of VariableValue structs.
func ToVariableValues(vars *map[string]interface{}) []VariableValue {
	if vars == nil {
		return nil
	}
	variables := make([]VariableValue, 0, len(*vars))
	for name, val := range *vars {
		v := VariableValue{Name: name}
		if varMap, ok := val.(map[string]interface{}); ok {
			if isSecret, ok := varMap["isSecret"].(bool); ok {
				v.IsSecret = isSecret
			}
			if value, ok := varMap["value"]; ok {
				v.Value = value
			}
		}
		variables = append(variables, v)
	}
	return variables
}

// FromVariableValues converts a slice of VariableValue structs back into a map for API consumption.
func FromVariableValues(variables []VariableValue) *map[string]interface{} {
	vars := make(map[string]interface{})
	for _, v := range variables {
		vars[v.Name] = map[string]interface{}{
			"value":    v.Value,
			"isSecret": v.IsSecret,
		}
	}
	return &vars
}

// GetVariableGroups issues a raw REST request that mirrors the Azure DevOps GetVariableGroups
// endpoint. The SDK's 7.1 wrapper currently mis-shapes the response body, so we decode the REST
// payload directly and hand the caller the continuation token alongside the variable groups.
func (c *extensionClient) GetVariableGroups(ctx context.Context, args taskagent.GetVariableGroupsArgs) (*VariableGroupsResponse, error) {
	if args.Project == nil || strings.TrimSpace(*args.Project) == "" {
		return nil, fmt.Errorf("project is required to list variable groups")
	}

	baseURL := strings.TrimRight(c.conn.BaseUrl, "/")
	project := url.PathEscape(strings.TrimSpace(*args.Project))
	requestURL := fmt.Sprintf("%s/%s/_apis/distributedtask/variablegroups", baseURL, project)

	query := url.Values{}
	query.Set("api-version", "7.1")
	if args.GroupName != nil && strings.TrimSpace(*args.GroupName) != "" {
		query.Set("groupName", strings.TrimSpace(*args.GroupName))
	}
	if args.ActionFilter != nil && string(*args.ActionFilter) != "" {
		query.Set("actionFilter", string(*args.ActionFilter))
	}
	if args.Top != nil {
		query.Set("$top", strconv.Itoa(*args.Top))
	}
	if args.ContinuationToken != nil {
		query.Set("continuationToken", strconv.Itoa(*args.ContinuationToken))
	}
	if args.QueryOrder != nil && string(*args.QueryOrder) != "" {
		query.Set("queryOrder", string(*args.QueryOrder))
	}

	if encoded := query.Encode(); encoded != "" {
		requestURL = fmt.Sprintf("%s?%s", requestURL, encoded)
	}

	client := c.conn.GetClientByUrl(requestURL)
	req, err := client.CreateRequestMessage(ctx, http.MethodGet, requestURL, "", nil, "", "", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.SendRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, client.UnwrapError(resp)
	}

	var payload VariableGroupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return &payload, nil
}
