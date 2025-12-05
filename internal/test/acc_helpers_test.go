package test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
)

// TestNullPrinter_NoOps verifies that calling nullPrinter methods doesn't panic
// and Render returns nil.
func TestNullPrinter_NoOps(t *testing.T) {
	p := &nullPrinter{}

	// Call each method with plausible inputs to ensure no panics and no errors.
	p.AddColumns("col1", "col2")
	p.AddField("value1")
	p.AddTimeField(time.Now(), time.Now(), func(s string) string { return s })
	p.EndRow()

	if err := p.Render(); err != nil {
		t.Fatalf("nullPrinter.Render() returned unexpected error: %v", err)
	}
}

// TestAcceptanceCmdContext_Printer ensures acceptanceCmdContext.Printer returns
// a non-nil nullPrinter without relying on external resources.
func TestAcceptanceCmdContext_Printer(t *testing.T) {
	ios, _, _, _ := iostreams.Test()
	ios.SetStdoutTTY(false)

	acc := &acceptanceCmdContext{
		baseCtx: context.Background(),
		ios:     ios,
		// other fields intentionally zero-valued because Printer does not use them
	}

	p, err := acc.Printer("anything")
	if err != nil {
		t.Fatalf("Printer returned unexpected error: %v", err)
	}
	if p == nil {
		t.Fatalf("Printer returned nil, expected non-nil printer")
	}

	// Render should succeed (nullPrinter Render returns nil).
	if err := p.Render(); err != nil {
		t.Fatalf("printer.Render returned unexpected error: %v", err)
	}
}

// TestTestContext_OrgFields verifies that testContext exposes Org, OrgUrl and PAT
// values set at construction time and that embedding a CmdContext does not
// prevent those accessors from working.
func TestTestContext_OrgFields(t *testing.T) {
	// Prepare unique values and set them with t.Setenv so they're restored automatically.
	org := "example-org-env"
	orgURL := "https://dev.azure.com/example-org-env"
	pat := "env-secret-pat"

	t.Setenv(accOrgEnv, org)
	t.Setenv(accPATEnv, pat)
	t.Setenv(accProjectEnv, "proj-value")

	// newTestContext validates env vars and returns a TestContext built from them.
	tc := newTestContext(t)

	// Verify that the TestContext accessors return the expected values from env.
	if got := tc.Org(); got != org {
		t.Fatalf("Org() = %q, want %q", got, org)
	}
	if got := tc.OrgUrl(); got != orgURL {
		t.Fatalf("OrgUrl() = %q, want %q", got, orgURL)
	}
	if got := tc.PAT(); got != pat {
		t.Fatalf("PAT() = %q, want %q", got, pat)
	}
	if got := tc.Project(); got != "proj-value" {
		t.Fatalf("Project() = %q, want proj-value", got)
	}
}

// TestNewTestContext_Config ensures that newTestContext builds a config from
// environment variables. It uses t.Setenv so env is restored automatically.
func TestNewTestContext_Config(t *testing.T) {
	// Prepare unique values
	org := "test-org-for-unit"
	orgURL := "https://dev.azure.com/test-org-for-unit"
	pat := "TEST_PAT_VALUE"

	// Use t.Setenv so the testing framework will automatically restore values.
	t.Setenv(accOrgEnv, org)
	t.Setenv(accPATEnv, pat)
	t.Setenv(accProjectEnv, "proj-alpha")

	// Call newTestContext which will build a config from the env vars.
	tc := newTestContext(t)

	// Retrieve the underlying config via TestContext.Config()
	cfg, err := tc.Config()
	if err != nil {
		t.Fatalf("failed to get config from TestContext: %v", err)
	}

	// Verify values in config match the env we set.
	gotURL, _ := cfg.Get([]string{config.Organizations, org, "url"})
	if gotURL != orgURL {
		t.Fatalf("config organizations.%s.url = %q, want %q", org, gotURL, orgURL)
	}
	gotPAT, _ := cfg.Get([]string{config.Organizations, org, "pat"})
	if gotPAT != pat {
		t.Fatalf("config organizations.%s.pat = %q, want %q", org, gotPAT, pat)
	}
}

// TestTestContextValueStore ensures SetValue/Value share data between steps.
func TestTestContextValueStore(t *testing.T) {
	t.Setenv(accOrgEnv, "org")
	t.Setenv(accPATEnv, "pat")
	t.Setenv(accProjectEnv, "project")

	tc := newTestContext(t)

	tc.SetValue("key", 42)
	if v, ok := tc.Value("missing"); ok || v != nil {
		t.Fatalf("Value for missing key should be absent, got %v", v)
	}
	if v, ok := tc.Value("key"); !ok || v.(int) != 42 {
		t.Fatalf("Value for key mismatch, got %v", v)
	}

	// Overwrite and ensure latest wins
	tc.SetValue("key", 99)
	if v, ok := tc.Value("key"); !ok || v.(int) != 99 {
		t.Fatalf("Value for key overwrite mismatch, got %v", v)
	}
}

// TestRunStep_PostRunAlwaysRuns verifies that PostRun runs even when Run returns an error.
func TestRunStep_PostRunAlwaysRuns_RunFails(t *testing.T) {
	called := struct {
		run     bool
		verify  bool
		postrun bool
	}{}

	step := Step{
		Run: func(ctx TestContext) error {
			called.run = true
			return errors.New("run failure")
		},
		Verify: func(ctx TestContext) error {
			called.verify = true
			return nil
		},
		PostRun: func(ctx TestContext) error {
			called.postrun = true
			return nil
		},
	}

	// Use a minimal testContext: newTestContext requires env vars, so craft a small stub.
	tc := &testContext{
		org:    "o",
		orgURL: "u",
		pat:    "p",
		CmdContext: &acceptanceCmdContext{
			baseCtx: nil,
			ios:     nil,
		},
	}

	err := runStep(tc, step)
	if err == nil {
		t.Fatalf("expected error from RunStep when Run fails")
	}
	// Ensure PostRun executed despite Run failing.
	if !called.postrun {
		t.Fatalf("PostRun was not executed when Run failed")
	}
}

// TestRunStep_PostRunAlwaysRuns verifies that PostRun runs even when Verify returns an error.
func TestRunStep_PostRunAlwaysRuns_VerifyFails(t *testing.T) {
	called := struct {
		run     bool
		verify  bool
		postrun bool
	}{}

	step := Step{
		Run: func(ctx TestContext) error {
			called.run = true
			return nil
		},
		Verify: func(ctx TestContext) error {
			called.verify = true
			return errors.New("verify failure")
		},
		PostRun: func(ctx TestContext) error {
			called.postrun = true
			return nil
		},
	}

	tc := &testContext{
		org:    "o",
		orgURL: "u",
		pat:    "p",
		CmdContext: &acceptanceCmdContext{
			baseCtx: nil,
			ios:     nil,
		},
	}

	err := runStep(tc, step)
	if err == nil {
		t.Fatalf("expected error from RunStep when Verify fails")
	}
	// Ensure PostRun executed despite Verify failing.
	if !called.postrun {
		t.Fatalf("PostRun was not executed when Verify failed")
	}
}

// TestRunStep_AggregatesErrors ensures that multiple errors are preserved/aggregated.
func TestRunStep_AggregatesErrors(t *testing.T) {
	step := Step{
		Run: func(ctx TestContext) error {
			return errors.New("run-err")
		},
		Verify: func(ctx TestContext) error {
			return errors.New("verify-err")
		},
		PostRun: func(ctx TestContext) error {
			return errors.New("post-err")
		},
	}

	tc := &testContext{
		org:    "o",
		orgURL: "u",
		pat:    "p",
		CmdContext: &acceptanceCmdContext{
			baseCtx: nil,
			ios:     nil,
		},
	}

	err := runStep(tc, step)
	if err == nil {
		t.Fatalf("expected aggregated error from RunStep")
	}

	// The aggregated error string should contain each individual message.
	msg := err.Error()
	if !strings.Contains(msg, "run-err") || !strings.Contains(msg, "post-err") {
		t.Fatalf("aggregated error missing components: %v", msg)
	}
}
