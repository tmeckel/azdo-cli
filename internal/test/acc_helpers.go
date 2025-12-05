package test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/prompter"
)

const (
	accToggleEnv      = "AZDO_ACC_TEST"
	accOrgEnv         = "AZDO_ACC_ORG"
	accPATEnv         = "AZDO_ACC_PAT"
	accTimeoutSeconds = 60
	accTimeoutEnv     = "AZDO_ACC_TIMEOUT"
	accProjectEnv     = "AZDO_ACC_PROJECT"
)

// nullPrinter is a no-op implementation of printer.Printer used for acceptance
// tests where formatted output is not needed. Methods are no-ops and Render() returns nil.
type nullPrinter struct{}

func (n *nullPrinter) AddColumns(columns ...string) {
}

func (n *nullPrinter) AddField(s string, _ ...printer.FieldOption) {
}

func (n *nullPrinter) AddTimeField(now, t time.Time, _ func(string) string) {
}

func (n *nullPrinter) EndRow() {
}

func (n *nullPrinter) Render() error {
	return nil
}

type TestCase struct {
	PreCheck func() error
	Steps    []Step
}

type TestContext interface {
	util.CmdContext
	Org() string
	OrgUrl() string
	PAT() string
	Project() string
	SetValue(key, value any)
	Value(key any) (any, bool)
}

type acceptanceCmdContext struct {
	baseCtx       context.Context
	ios           *iostreams.IOStreams
	connFactory   azdo.ConnectionFactory
	cfg           config.Config
	clientFactory azdo.ClientFactory
	prompter      prompter.Prompter
}

var _ util.CmdContext = (*acceptanceCmdContext)(nil)

func (a *acceptanceCmdContext) Context() context.Context                  { return a.baseCtx }
func (a *acceptanceCmdContext) RepoContext() util.RepoContext             { return nil }
func (a *acceptanceCmdContext) ConnectionFactory() azdo.ConnectionFactory { return a.connFactory }
func (a *acceptanceCmdContext) ClientFactory() azdo.ClientFactory         { return a.clientFactory }
func (a *acceptanceCmdContext) Prompter() (prompter.Prompter, error) {
	if a.prompter == nil {
		a.prompter = &stubPrompter{}
	}
	return a.prompter, nil
}
func (a *acceptanceCmdContext) Config() (config.Config, error)           { return a.cfg, nil }
func (a *acceptanceCmdContext) IOStreams() (*iostreams.IOStreams, error) { return a.ios, nil }
func (a *acceptanceCmdContext) Printer(string) (printer.Printer, error) {
	return &nullPrinter{}, nil
}

type testContext struct {
	org     string
	orgURL  string
	pat     string
	project string
	data    sync.Map
	util.CmdContext
}

// Ensure testContext implements util.CmdContext by delegation.
var _ util.CmdContext = (*testContext)(nil)

func (tc *testContext) Org() string {
	return tc.org
}

func (tc *testContext) OrgUrl() string {
	return tc.orgURL
}

func (tc *testContext) PAT() string {
	return tc.pat
}

func (tc *testContext) Project() string {
	return tc.project
}

func (tc *testContext) SetValue(key, value any) {
	if key == nil {
		return
	}
	tc.data.Store(key, value)
}

func (tc *testContext) Value(key any) (any, bool) {
	if key == nil {
		return nil, false
	}
	return tc.data.Load(key)
}

// Precheck and context builder
func newTestContext(t *testing.T) TestContext {
	org := os.Getenv(accOrgEnv)
	pat := os.Getenv(accPATEnv)
	project := os.Getenv(accProjectEnv)

	if org == "" || pat == "" {
		t.Fatalf("missing acceptance env variables: %q, %q", accOrgEnv, accPATEnv)
	}

	orgurl := fmt.Sprintf("https://dev.azure.com/%s", org)

	// Build a safe YAML configuration using marshaling instead of fmt.Sprintf interpolation.
	// This avoids accidental YAML-breaking characters in env values.
	cfgData := map[string]any{
		"organizations": map[string]any{
			org: map[string]any{
				"url":          orgurl,
				"pat":          pat,
				"git_protocol": "https",
			},
		},
	}
	cfgBytes, err := yaml.Marshal(cfgData)
	if err != nil {
		t.Fatalf("failed to marshal config YAML: %v", err)
	}
	cfgRdr, err := config.NewStringConfigReader(string(cfgBytes))
	if err != nil {
		t.Fatalf("failed to create ConfigReader %v", err)
	}

	cfg, err := config.NewConfigWithReader(cfgRdr)
	if err != nil {
		t.Fatalf("failed to create config %v", err)
	}
	auth, err := util.NewPatAuthenticator(cfg)
	if err != nil {
		t.Fatalf("failed to create PAT authenticator %v", err)
	}

	connFactory, err := azdo.NewConnectionFactory(cfg, auth)
	if err != nil {
		t.Fatalf("failed to create azdo connection factory: %v", err)
	}
	clientFactory, err := azdo.NewClientFactory(connFactory)
	if err != nil {
		t.Fatalf("failed to create azdo client factory: %v", err)
	}
	ios, _, _, _ := iostreams.Test()
	ios.SetStdoutTTY(false)
	// Determine timeout: default accTimeoutSeconds, but allow override via AZDO_ACC_TIMEOUT.
	// Accept full duration strings (e.g., "30s", "1m") or a plain integer interpreted as seconds for backwards compatibility.
	var baseCtx context.Context
	var cancel context.CancelFunc

	timeoutVal := os.Getenv(accTimeoutEnv)
	debugVal := os.Getenv("AZDO_DEBUG")

	if timeoutVal == "-1" || debugVal == "1" {
		baseCtx, cancel = context.WithCancel(context.Background())
	} else {
		timeoutSec := accTimeoutSeconds
		if timeoutVal != "" {
			// Try parsing as a full duration first.
			if parsedDur, err := time.ParseDuration(timeoutVal); err == nil {
				if parsedDur <= 0 {
					t.Fatalf("invalid %s value '%s' — duration must be > 0", accTimeoutEnv, timeoutVal)
				}
				timeoutSec = int(parsedDur / time.Second)
			} else {
				// Backwards-compatible integer-only parse (seconds).
				var pi int
				if _, err2 := fmt.Sscanf(timeoutVal, "%d", &pi); err2 == nil && pi > 0 {
					timeoutSec = pi
				} else {
					t.Fatalf("invalid %s value '%s' — provide a duration (e.g. \"30s\") or a positive integer (seconds)", accTimeoutEnv, timeoutVal)
				}
			}
		}
		baseCtx, cancel = context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	}
	t.Cleanup(cancel)

	return &testContext{
		org:     org,
		orgURL:  orgurl,
		pat:     pat,
		project: strings.TrimSpace(project),
		CmdContext: &acceptanceCmdContext{
			baseCtx:       baseCtx,
			ios:           ios,
			cfg:           cfg,
			connFactory:   connFactory,
			clientFactory: clientFactory,
			prompter:      &stubPrompter{},
		},
	}
}

type stubPrompter struct{}

func (stubPrompter) Select(string, string, []string) (int, error) {
	return 0, fmt.Errorf("interactive prompts are disabled in acceptance tests")
}

func (stubPrompter) MultiSelect(string, []string, []string) ([]int, error) {
	return nil, fmt.Errorf("interactive prompts are disabled in acceptance tests")
}

func (stubPrompter) Input(string, string) (string, error) {
	return "", fmt.Errorf("interactive prompts are disabled in acceptance tests")
}

func (stubPrompter) InputOrganizationName() (string, error) {
	return "", fmt.Errorf("interactive prompts are disabled in acceptance tests")
}

func (stubPrompter) Password(string) (string, error) {
	return "", fmt.Errorf("interactive prompts are disabled in acceptance tests")
}

func (stubPrompter) AuthToken() (string, error) {
	return "", fmt.Errorf("interactive prompts are disabled in acceptance tests")
}

func (stubPrompter) Confirm(string, bool) (bool, error) {
	return true, nil
}

func (stubPrompter) ConfirmDeletion(string) error {
	return nil
}

// Compact acc runner
type Step struct {
	PreRun  func(TestContext) error
	Run     func(TestContext) error
	PostRun func(TestContext) error
	Verify  func(TestContext) error
}

// RunStep executes a single Step and returns an aggregated error.
// It guarantees PostRun is executed regardless of Run or Verify outcome.
func runStep(ctx TestContext, s Step) error {
	var errs []error

	if s.PreRun != nil {
		if err := s.PreRun(ctx); err != nil {
			return fmt.Errorf("pre: %w", err)
		}
	}

	if s.Run != nil {
		if err := s.Run(ctx); err != nil {
			errs = append(errs, fmt.Errorf("run: %w", err))
		}
	}

	if s.Verify != nil && len(errs) == 0 {
		if err := s.Verify(ctx); err != nil {
			errs = append(errs, fmt.Errorf("verify: %w", err))
		}
	}

	// Always attempt PostRun cleanup regardless of Run/Verify outcome.
	if s.PostRun != nil {
		if err := s.PostRun(ctx); err != nil {
			errs = append(errs, fmt.Errorf("post: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func Test(t *testing.T, tc TestCase) {
	if os.Getenv(accToggleEnv) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", accToggleEnv)
		return
	}

	if tc.PreCheck != nil {
		if err := tc.PreCheck(); err != nil {
			t.Fatalf("test PreCheck failed: %v", err)
		}
	}
	ctx := newTestContext(t)
	for _, s := range tc.Steps {
		if err := runStep(ctx, s); err != nil {
			t.Fatalf("%v", err)
		}
	}
}
