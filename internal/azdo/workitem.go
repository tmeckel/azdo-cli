package azdo

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/workitemtracking"
)

// workItemLocationID and workItemAPIVersion mirror the request issued by
// workitemtracking.Client.GetWorkItem so the raw payload matches the SDK
// response for the same work item.
var (
	workItemLocationID = uuid.MustParse("72c7ddf8-2cdc-4f60-90cd-ab71c14a399b")
	workItemAPIVersion = "7.1-preview.3"
)

// WorkItemEnvelope is the raw work item payload. The vendored SDK WorkItem
// model drops the unknown multilineFieldsFormat property, which commands need
// to render descriptions format-aware.
type WorkItemEnvelope struct {
	workitemtracking.WorkItem
	MultilineFieldsFormat *map[string]string `json:"multilineFieldsFormat,omitempty"`
}

// GetWorkItemEnvelope fetches the raw work item payload for the given ID via
// the low-level client, capturing properties the SDK model drops.
func GetWorkItemEnvelope(ctx context.Context, client Client, project string, id int, expand *workitemtracking.WorkItemExpand) (*WorkItemEnvelope, error) {
	routeValues := map[string]string{"id": strconv.Itoa(id)}
	if project != "" {
		routeValues["project"] = project
	}
	queryParams := url.Values{}
	if expand != nil {
		queryParams.Add("$expand", string(*expand))
	}

	resp, err := client.Send(ctx, http.MethodGet, workItemLocationID, workItemAPIVersion, routeValues, queryParams, nil, "", "application/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var envelope WorkItemEnvelope
	if err := client.UnmarshalBody(resp, &envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}
