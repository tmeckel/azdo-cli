package util

import (
	"context"
	"os"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/git"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/prompter"
)

type CmdContext interface {
	Prompter() (prompter.Prompter, error)
	Context() (context.Context, error)
	Config() (config.Config, error)
	Connection(organization string) (*azuredevops.Connection, error)
	IOStreams() (*iostreams.IOStreams, error)
	Printer(string) (printer.Printer, error)
	GitClient() (*git.Client, error)
}

type cmdContext struct {
	ioStreams *iostreams.IOStreams
	prompter  prompter.Prompter
	ctx       context.Context
	cfg       config.Config
	auth      Authenticator
}

func NewCmdContext() (ctx CmdContext, err error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return
	}
	auth, err := NewPatAuthenticator(cfg)
	if err != nil {
		return
	}

	iostrms, err := newIOStreams(cfg)
	if err != nil {
		return
	}

	p, err := newPrompter(cfg, iostrms)
	if err != nil {
		return
	}

	ctx = &cmdContext{
		ioStreams: iostrms,
		prompter:  p,
		ctx:       context.Background(),
		cfg:       cfg,
		auth:      auth,
	}
	return
}

func (c *cmdContext) Prompter() (p prompter.Prompter, err error) {
	p = c.prompter
	return
}

func (c *cmdContext) Context() (ctx context.Context, err error) {
	ctx = c.ctx
	return
}

func (c *cmdContext) Config() (cfg config.Config, err error) {
	cfg = c.cfg
	return
}

func (c *cmdContext) Connection(organization string) (client *azuredevops.Connection, err error) {
	organizationURL, err := c.cfg.Get([]string{config.Organizations, organization, "url"})
	if err != nil {
		return
	}

	authHrd, err := c.auth.GetAuthorizationHeader(organization)
	if err != nil {
		return
	}
	client = &azuredevops.Connection{
		AuthorizationString:     authHrd,
		BaseUrl:                 strings.ToLower(strings.TrimRight(organizationURL, "/")),
		SuppressFedAuthRedirect: true,
	}
	return
}

func (c *cmdContext) IOStreams() (*iostreams.IOStreams, error) {
	return c.ioStreams, nil
}

func (c *cmdContext) Printer(t string) (p printer.Printer, err error) {
	switch t {
	case "tsv":
		p, err = newTablePrinter(c.ioStreams)
	case "json":
		p, err = printer.NewJsonPrinter(c.ioStreams.Out)
	default:
		return nil, printer.NewUnsupportedPrinterError(t)
	}
	return
}

func (c *cmdContext) GitClient() (client *git.Client, err error) {
	client, err = newGitClient(c.ioStreams)
	return
}

func newGitClient(io *iostreams.IOStreams) (client *git.Client, err error) {
	azdoPath, err := os.Executable()
	if err != nil {
		return
	}
	client = &git.Client{
		AzDoPath: azdoPath,
		Stderr:   io.ErrOut,
		Stdin:    io.In,
		Stdout:   io.Out,
	}
	return
}

func newIOStreams(cfg config.Config) (*iostreams.IOStreams, error) {
	io := iostreams.System()

	if _, ghPromptDisabled := os.LookupEnv("AZDO_PROMPT_DISABLED"); ghPromptDisabled {
		io.SetNeverPrompt(true)
	} else if prompt, _ := cfg.GetOrDefault([]string{config.Organizations, "", "prompt"}); prompt == "disabled" {
		io.SetNeverPrompt(true)
	}

	// Pager precedence
	// 1. AZDO_PAGER
	// 2. pager from config
	// 3. PAGER
	if ghPager, ghPagerExists := os.LookupEnv("AZDO_PAGER"); ghPagerExists {
		io.SetPager(ghPager)
	} else if pager, _ := cfg.Get([]string{config.Organizations, "", "pager"}); pager != "" {
		io.SetPager(pager)
	}

	return io, nil
}

func newPrompter(cfg config.Config, io *iostreams.IOStreams) (p prompter.Prompter, err error) {
	editor, err := config.DetermineEditor(cfg)
	if err != nil {
		return
	}
	p = prompter.New(editor, io.In, io.Out, io.ErrOut)
	return
}

func newTablePrinter(ios *iostreams.IOStreams) (printer.TablePrinter, error) {
	maxWidth := 80
	isTTY := ios.IsStdoutTTY()
	if isTTY {
		maxWidth = ios.TerminalWidth()
	}
	return printer.NewTablePrinter(ios.Out, isTTY, maxWidth)
}
