package list_test

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "testing"

    "github.com/microsoft/azure-devops-go-api/azuredevops/v7"
    "github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/tmeckel/azdo-cli/internal/types"
)

// Test configuration via environment variables. Tests will be skipped when not provided.
// Preferred variables:
// - AZDO_TEST_ORGANIZATION or fallback AZDO_ORGANIZATION
// - AZDO_TEST_PAT or fallback AZDO_TOKEN
// - AZDO_TEST_EMAIL for subject lookup

func getOrganization(t *testing.T) string {
    t.Helper()
    if v := os.Getenv("AZDO_TEST_ORGANIZATION"); v != "" {
        return v
    }
    if v := os.Getenv("AZDO_ORGANIZATION"); v != "" {
        return v
    }
    t.Skip("set AZDO_TEST_ORGANIZATION or AZDO_ORGANIZATION to run this test")
    return ""
}

func getOrganizationURL(t *testing.T) string {
    return fmt.Sprintf("https://dev.azure.com/%s", getOrganization(t))
}

func getPAT(t *testing.T) string {
    t.Helper()
    if v := os.Getenv("AZDO_TEST_PAT"); v != "" {
        return v
    }
    if v := os.Getenv("AZDO_TOKEN"); v != "" {
        return v
    }
    t.Skip("set AZDO_TEST_PAT or AZDO_TOKEN to run this test")
    return ""
}

func getConnection(t *testing.T) *azuredevops.Connection {
    return &azuredevops.Connection{
        AuthorizationString:     azuredevops.CreateBasicAuthHeaderValue("", getPAT(t)),
        BaseUrl:                 getOrganizationURL(t),
        SuppressFedAuthRedirect: true,
    }
}

func TestConnectionData(t *testing.T) {
    conn := getConnection(t)
    connectionDataURL := fmt.Sprintf("%s/_apis/connectionData", conn.BaseUrl)
    connectionDataClient := conn.GetClientByUrl(connectionDataURL)
    request, err := connectionDataClient.CreateRequestMessage(context.Background(), http.MethodGet, connectionDataURL, "", nil, "", "", nil)

    require.NoError(t, err)
    response, err := connectionDataClient.SendRequest(request)
    require.NoError(t, err)
    defer response.Body.Close()

    var jsonData map[string]any
    err = json.NewDecoder(response.Body).Decode(&jsonData)
    if err != nil {
        bodyBytes, err := io.ReadAll(response.Body)
        require.NoError(t, err)
        bodyString := string(bodyBytes)
        _, _ = os.Stderr.WriteString(bodyString)
    }
}

func TestQuerySubsjects(t *testing.T) {
    conn := getConnection(t)
    email := os.Getenv("AZDO_TEST_EMAIL")
    if email == "" {
        t.Skip("set AZDO_TEST_EMAIL to run this test")
    }

    ctx := context.Background()

    graphClient, err := graph.NewClient(ctx, conn)
    require.NoError(t, err)

    subjects, err := graphClient.QuerySubjects(ctx, graph.QuerySubjectsArgs{
        SubjectQuery: &graph.GraphSubjectQuery{
            Query: types.ToPtr(email),
            SubjectKind: &[]string{
                "User",
            },
        },
    })
    require.NoError(t, err)
    assert.NotEmpty(t, subjects)
}
