package list

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
	"github.com/tmeckel/azdo-cli/internal/printer"
)

type areaListDeps struct {
	ctrl       *gomock.Controller
	cmd        *mocks.MockCmdContext
	connFact   *mocks.MockConnectionFactory
	conn       *mocks.MockConnection
	client     *mocks.MockClient
	authCfg    *mocks.MockAuthConfig
	orgURL     string
	defaultOrg string
}

func newAreaListDeps(t *testing.T, organization, orgURL string) *areaListDeps {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(false)
	io.SetStderrTTY(false)

	tp, err := printer.NewTablePrinter(out, false, 200)
	require.NoError(t, err)

	deps := &areaListDeps{
		ctrl:     ctrl,
		cmd:      mocks.NewMockCmdContext(ctrl),
		connFact: mocks.NewMockConnectionFactory(ctrl),
		conn:     mocks.NewMockConnection(ctrl),
		client:   mocks.NewMockClient(ctrl),
		authCfg:  mocks.NewMockAuthConfig(ctrl),
		orgURL:   orgURL,
	}

	deps.cmd.EXPECT().IOStreams().Return(io, nil).AnyTimes()
	deps.cmd.EXPECT().Context().Return(context.Background()).AnyTimes()
	deps.cmd.EXPECT().ConnectionFactory().Return(deps.connFact).AnyTimes()
	deps.cmd.EXPECT().Printer("table").Return(tp, nil).AnyTimes()

	cfg := mocks.NewMockConfig(ctrl)
	deps.cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(deps.authCfg).AnyTimes()
	deps.authCfg.EXPECT().GetURL(gomock.Any()).DoAndReturn(func(org string) (string, error) {
		return orgURL, nil
	}).AnyTimes()

	return deps
}

func (d *areaListDeps) expectOrg(organization string) {
	d.connFact.EXPECT().Connection(organization).Return(d.conn, nil).AnyTimes()
	d.conn.EXPECT().GetClientByUrl(d.orgURL).Return(d.client).AnyTimes()
}

func (d *areaListDeps) expectList(t *testing.T, project string, wantURLContains []string) {
	d.client.EXPECT().CreateRequestMessage(gomock.Any(), http.MethodGet, gomock.Any(), "7.1", nil, "", gomock.Any(), nil).
		DoAndReturn(func(ctx context.Context, httpMethod, reqURL, apiVersion string, body io.Reader, mediaType, acceptMediaType string, additionalHeaders map[string]string) (*http.Request, error) {
			for _, want := range wantURLContains {
				assert.Contains(t, reqURL, want)
			}
			req, err := http.NewRequestWithContext(ctx, httpMethod, reqURL, body)
			require.NoError(t, err)
			return req, nil
		})
	d.client.EXPECT().SendRequest(gomock.Any()).DoAndReturn(func(req *http.Request) (*http.Response, error) {
		payload := `{"id":1,"name":"Fabrikam","path":"Fabrikam","hasChildren":false,"children":[{"id":2,"name":"Web","path":"Fabrikam\\Web","hasChildren":false}]}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload))}, nil
	})
	d.client.EXPECT().UnmarshalBody(gomock.Any(), gomock.Any()).DoAndReturn(func(resp *http.Response, v any) error {
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, v)
	})
}

func TestList_ExplicitOrgRouting(t *testing.T) {
	t.Parallel()

	deps := newAreaListDeps(t, "myorg", "https://dev.azure.com/myorg")
	deps.expectOrg("myorg")
	deps.expectList(t, "Fabrikam", []string{"/myorg/Fabrikam/_apis/wit/classificationnodes/Areas"})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myorg:Fabrikam"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_DefaultOrgRouting(t *testing.T) {
	t.Parallel()

	deps := newAreaListDeps(t, "default-org", "https://dev.azure.com/default-org")
	deps.authCfg.EXPECT().GetDefaultOrganization().Return("default-org", nil).AnyTimes()
	deps.expectOrg("default-org")
	deps.expectList(t, "Fabrikam", []string{"/default-org/Fabrikam/_apis/wit/classificationnodes/Areas"})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"Fabrikam"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_LegacyOrgSlashIsRejected(t *testing.T) {
	t.Parallel()

	// A legacy ORGANIZATION/PROJECT input carries no ORG: prefix and is
	// structurally detectable in this no-target mode, so it must be rejected
	// with ORG: guidance instead of being reinterpreted.
	deps := newAreaListDeps(t, "myorg", "https://dev.azure.com/myorg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myorg/Fabrikam"})
	err := cmd.Execute()
	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), "legacy ORGANIZATION/... form is not supported, use ORG: syntax")
}

func TestList_RendersTable(t *testing.T) {
	t.Parallel()

	deps := newAreaListDeps(t, "myorg", "https://dev.azure.com/myorg")
	deps.expectOrg("myorg")
	deps.expectList(t, "Fabrikam", []string{"/myorg/Fabrikam/_apis/wit/classificationnodes/Areas"})

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myorg:Fabrikam"})
	err := cmd.Execute()
	require.NoError(t, err)
}

func TestList_ConnectionError(t *testing.T) {
	t.Parallel()

	deps := newAreaListDeps(t, "myorg", "https://dev.azure.com/myorg")
	deps.connFact.EXPECT().Connection("myorg").Return(nil, errors.New("boom"))

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myorg:Fabrikam"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create connection")
}

func TestList_InvalidDepth(t *testing.T) {
	t.Parallel()

	deps := newAreaListDeps(t, "myorg", "https://dev.azure.com/myorg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{"myorg:Fabrikam", "--depth", "-1"})
	err := cmd.Execute()
	require.Error(t, err)
	var flagErr *util.FlagError
	require.ErrorAs(t, err, &flagErr)
	assert.Contains(t, err.Error(), "--depth must be greater than or equal to 0")
}

func TestList_EmptyScope(t *testing.T) {
	t.Parallel()

	deps := newAreaListDeps(t, "myorg", "https://dev.azure.com/myorg")

	cmd := NewCmd(deps.cmd)
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project argument required")
}
