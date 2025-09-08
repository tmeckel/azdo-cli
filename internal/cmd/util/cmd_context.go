package util

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/security"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/git"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/prompter"
	"github.com/tmeckel/azdo-cli/internal/util"
)

type RepoContext interface {
	util.ContextAware
	GitCommand() (git.GitCommand, error)
	Remotes() (azdo.RemoteSet, error)
	Remote(*azdogit.GitRepository) (*azdo.Remote, error)
	WithRepo(func() (azdo.Repository, error)) RepoContext
	Repo() (azdo.Repository, error)
	GitClient() (azdogit.Client, error)
	GitRepository() (*azdogit.GitRepository, error)
}

type CmdContext interface {
	util.ContextAware
	RepoContext() RepoContext
	ConnectionFactory() azdo.ConnectionFactory
	Prompter() (prompter.Prompter, error)
	Config() (config.Config, error)
	IOStreams() (*iostreams.IOStreams, error)
	Printer(string) (printer.Printer, error)
}

type cmdContext struct {
	ioStreams    *iostreams.IOStreams
	prompter     prompter.Prompter
	ctx          context.Context
	cfg          config.Config
	auth         Authenticator
	repoOverride func() (azdo.Repository, error)
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

	c := &cmdContext{
		ioStreams: iostrms,
		prompter:  p,
		ctx:       context.Background(),
		cfg:       cfg,
		auth:      auth,
	}
	ctx = c
	return
}

func (c *cmdContext) Prompter() (p prompter.Prompter, err error) {
	p = c.prompter
	return
}

func (c *cmdContext) Context() (ctx context.Context) {
	ctx = c.ctx
	return
}

func (c *cmdContext) RepoContext() RepoContext {
	return c
}

func (c *cmdContext) ConnectionFactory() azdo.ConnectionFactory {
	return c
}

func (c *cmdContext) Git(ctx context.Context, org string) (azdogit.Client, error) {
	conn, err := c.Connection(org)
	if err != nil {
		return nil, err
	}
	return azdogit.NewClient(ctx, conn)
}

func (c *cmdContext) Identity(ctx context.Context, org string) (identity.Client, error) {
	conn, err := c.Connection(org)
	if err != nil {
		return nil, err
	}
	return identity.NewClient(ctx, conn)
}

func (c *cmdContext) Graph(ctx context.Context, org string) (graph.Client, error) {
	conn, err := c.Connection(org)
	if err != nil {
		return nil, err
	}
	return graph.NewClient(ctx, conn)
}

func (c *cmdContext) Core(ctx context.Context, org string) (core.Client, error) {
	conn, err := c.Connection(org)
	if err != nil {
		return nil, err
	}
	return core.NewClient(ctx, conn)
}

func (c *cmdContext) Security(ctx context.Context, org string) (security.Client, error) {
	conn, err := c.Connection(org)
	if err != nil {
		return nil, err
	}
	return security.NewClient(ctx, conn), nil
}

func (c *cmdContext) GitClient() (azdogit.Client, error) {
	repo, err := c.Repo()
	if err != nil {
		return nil, err
	}
	conn, err := c.Connection(repo.Organization())
	if err != nil {
		return nil, err
	}
	gitClient, err := azdogit.NewClient(c.Context(), conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create new Git client: %w", err)
	}
	return gitClient, nil
}

func (c *cmdContext) GitRepository() (*azdogit.GitRepository, error) {
	repo, err := c.Repo()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo: %w", err)
	}
	gitClient, err := c.GitClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Git client: %w", err)
	}
	return repo.GitRepository(c.Context(), gitClient)
}

func (c *cmdContext) Config() (cfg config.Config, err error) {
	cfg = c.cfg
	return
}

func (c *cmdContext) Connection(organization string) (client *azuredevops.Connection, err error) {
	organization = strings.ToLower(organization)
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
	case "table":
		p, err = newTablePrinter(c.ioStreams)
	case "json":
		p, err = printer.NewJSONPrinter(c.ioStreams.Out)
	default:
		return nil, printer.NewUnsupportedPrinterError(t)
	}
	return
}

func (c *cmdContext) Remotes() (remotes azdo.RemoteSet, err error) {
	client, err := c.GitCommand()
	if err != nil {
		return
	}
	remoteSet, err := client.Remotes(c.ctx)
	if err != nil {
		return
	}
	remotes = azdo.TranslateRemotes(remoteSet, azdo.NewIdentityTranslator())
	return
}

func (c *cmdContext) Remote(repo *azdogit.GitRepository) (remote *azdo.Remote, err error) {
	remotes, err := c.Remotes()
	if err != nil {
		return
	}
	url := *repo.RemoteUrl
	sshUrl := *repo.SshUrl

	for _, r := range remotes {
		if (r.FetchURL.String() == url || r.FetchURL.String() == sshUrl) ||
			(r.PushURL.String() == url || r.PushURL.String() == sshUrl) {
			remote = r
			return
		}
	}
	return
}

func (c *cmdContext) GitCommand() (client git.GitCommand, err error) {
	client, err = git.NewGitCommand(c.ioStreams)
	return
}

func (c *cmdContext) WithRepo(override func() (azdo.Repository, error)) RepoContext {
	c.repoOverride = override
	return c
}

func (c *cmdContext) Repo() (result azdo.Repository, err error) {
	if c.repoOverride != nil {
		return c.repoOverride()
	}
	remotes, err := c.Remotes()
	if err != nil {
		return nil, err
	}
	defaultRemote, err := remotes.DefaultRemote()
	if err != nil {
		return
	}
	result = defaultRemote.Repository()
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
	pt, err := printer.NewTablePrinter(ios.Out, isTTY, maxWidth)
	if err != nil {
		return nil, fmt.Errorf("failed to create new table printer: %w", err)
	}
	return pt, nil
}
