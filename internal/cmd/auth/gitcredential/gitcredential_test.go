package gitcredential

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/mocks"
)

func TestHelperRun_get(t *testing.T) {
	t.Parallel()

	const token = "secret-token"

	tests := []struct {
		name       string
		input      string
		tokenFor   string   // organization GetToken is expected for; "" = no lookup
		tokenValue string   // token returned by GetToken
		wantErr    string   // expected error text; "" = success
		wantOutput []string // substrings expected in stdout on success
	}{
		{
			name:       "visualstudio.com host extracts organization from subdomain",
			input:      "protocol=https\nhost=vsorg.visualstudio.com\npath=/monalisa/_git/octo-cat\n\n",
			tokenFor:   "vsorg",
			tokenValue: token,
			wantOutput: []string{"protocol=https", "host=vsorg.visualstudio.com", "password=secret-token"},
		},
		{
			name:       "dev.azure.com host extracts organization from path",
			input:      "protocol=https\nhost=dev.azure.com\npath=/defaultorg/monalisa/_git/octo-cat\n\n",
			tokenFor:   "defaultorg",
			tokenValue: token,
			wantOutput: []string{"protocol=https", "host=dev.azure.com", "password=secret-token"},
		},
		{
			name:    "dev.azure.com host without path",
			input:   "protocol=https\nhost=dev.azure.com\n\n",
			wantErr: "authenticating via dev.azure.com host requires path parameter",
		},
		{
			name:    "non-Azure DevOps host",
			input:   "protocol=https\nhost=github.com\npath=/owner/repo\n\n",
			wantErr: "not an Azure DevOps host github.com",
		},
		{
			name:    "protocol not https",
			input:   "protocol=git\nhost=dev.azure.com\npath=/defaultorg\n\n",
			wantErr: "protocol git != https",
		},
		{
			name:       "token missing for organization",
			input:      "protocol=https\nhost=dev.azure.com\npath=/defaultorg\n\n",
			tokenFor:   "defaultorg",
			tokenValue: "",
			wantErr:    "unable to get token for organization defaultorg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			ios, in, out, _ := iostreams.Test()
			_, err := in.WriteString(tt.input)
			require.NoError(t, err)

			cmd := mocks.NewMockCmdContext(ctrl)
			cmd.EXPECT().IOStreams().Return(ios, nil).AnyTimes()

			cfg := mocks.NewMockConfig(ctrl)
			auth := mocks.NewMockAuthConfig(ctrl)
			cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
			cfg.EXPECT().Authentication().Return(auth).AnyTimes()

			if tt.tokenFor != "" {
				auth.EXPECT().GetToken(tt.tokenFor).Return(tt.tokenValue, nil).AnyTimes()
			}

			err = helperRun(cmd, &credentialOptions{operation: "get"})
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			for _, want := range tt.wantOutput {
				assert.Contains(t, out.String(), want)
			}
			assert.Contains(t, out.String(), "username=")
		})
	}
}

func TestHelperRun_urlLine(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	ios, in, out, _ := iostreams.Test()
	_, err := in.WriteString("url=https://vsorg.visualstudio.com/monalisa/_git/octo-cat\n\n")
	require.NoError(t, err)

	cmd := mocks.NewMockCmdContext(ctrl)
	cmd.EXPECT().IOStreams().Return(ios, nil).AnyTimes()
	cfg := mocks.NewMockConfig(ctrl)
	auth := mocks.NewMockAuthConfig(ctrl)
	cmd.EXPECT().Config().Return(cfg, nil).AnyTimes()
	cfg.EXPECT().Authentication().Return(auth).AnyTimes()
	auth.EXPECT().GetToken("vsorg").Return("secret-token", nil).AnyTimes()

	require.NoError(t, helperRun(cmd, &credentialOptions{operation: "get"}))
	assert.Contains(t, out.String(), "password=secret-token")
}
