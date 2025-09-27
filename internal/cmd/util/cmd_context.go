package util

import (
	"context"
	"fmt"
	"net/url"
	"os"

	azdogit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/config"
	"github.com/tmeckel/azdo-cli/internal/git"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/prompter"
	"github.com/tmeckel/azdo-cli/internal/util"
	"go.uber.org/zap"
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
	ClientFactory() azdo.ClientFactory
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
	auth         azdo.Authenticator
	repoOverride func() (azdo.Repository, error)
}

func NewCmdContext() (ctx CmdContext, err error) {
	cfg, err := config.NewConfig()
	if err != nil {
		return ctx, err
	}
	auth, err := NewPatAuthenticator(cfg)
	if err != nil {
		return ctx, err
	}

	iostrms, err := newIOStreams(cfg)
	if err != nil {
		return ctx, err
	}

	p, err := newPrompter(cfg, iostrms)
	if err != nil {
		return ctx, err
	}

	c := &cmdContext{
		ioStreams: iostrms,
		prompter:  p,
		ctx:       context.Background(),
		cfg:       cfg,
		auth:      auth,
	}
	ctx = c
	return ctx, err
}

func (c *cmdContext) Prompter() (p prompter.Prompter, err error) {
	p = c.prompter
	return p, err
}

func (c *cmdContext) Context() (ctx context.Context) {
	ctx = c.ctx
	return ctx
}

func (c *cmdContext) RepoContext() RepoContext {
	return c
}

func (c *cmdContext) ConnectionFactory() azdo.ConnectionFactory {
	fac, err := azdo.NewConnectionFactory(c.cfg, c.auth)
	if err != nil {
		panic(err)
	}
	return fac
}

func (c *cmdContext) ClientFactory() azdo.ClientFactory {
	fac, err := azdo.NewClientFactory(c.ConnectionFactory())
	if err != nil {
		panic(err)
	}
	return fac
}

func (c *cmdContext) Config() (cfg config.Config, err error) {
	cfg = c.cfg
	return cfg, err
}

func (c *cmdContext) GitClient() (azdogit.Client, error) {
	repo, err := c.Repo()
	if err != nil {
		return nil, err
	}
	clientFactory, err := azdo.NewClientFactory(c.ConnectionFactory())
	if err != nil {
		return nil, err
	}
	gitClient, err := clientFactory.Git(c.Context(), repo.Organization())
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
	return p, err
}

func (c *cmdContext) Remotes() (remotes azdo.RemoteSet, err error) {
	client, err := c.GitCommand()
	if err != nil {
		return remotes, err
	}
	remoteSet, err := client.Remotes(c.ctx)
	if err != nil {
		return remotes, err
	}
	remotes = azdo.TranslateRemotes(remoteSet, azdo.NewIdentityTranslator())
	return remotes, err
}

func (c *cmdContext) Remote(repo *azdogit.GitRepository) (remote *azdo.Remote, err error) {
	remotes, err := c.Remotes()
	if err != nil {
		return remote, err
	}

	if len(remotes) == 0 {
		err = fmt.Errorf("no git remotes found for repository %q", *repo.Name)
		return remote, err
	}

	urlComp := util.NewURLComparer()

	var url, sshUrl *url.URL

	if repo.RemoteUrl != nil {
		url, err = url.Parse(*repo.RemoteUrl)
		if err != nil {
			err = fmt.Errorf("failed to parse remote URL %q: %w", *repo.RemoteUrl, err)
			return remote, err
		}
	}
	if repo.SshUrl != nil {
		url, err = url.Parse(*repo.SshUrl)
		if err != nil {
			err = fmt.Errorf("failed to parse SSH URL %q: %w", *repo.SshUrl, err)
			return remote, err
		}
	}
	for _, r := range remotes {
		zap.L().Sugar().Debugf("Checking remote %+v for match with URL %q or %q", r, url, sshUrl)
		if (urlComp.EqualURLs(r.FetchURL, url) || urlComp.EqualURLs(r.FetchURL, sshUrl)) ||
			(urlComp.EqualURLs(r.PushURL, url) || urlComp.EqualURLs(r.PushURL, sshUrl)) {
			remote = r
			return remote, err
		}
	}
	err = fmt.Errorf("no remote found for repository %q", *repo.Name)
	return remote, err
}

func (c *cmdContext) GitCommand() (client git.GitCommand, err error) {
	client, err = git.NewGitCommand(c.ioStreams)
	return client, err
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
		return result, err
	}
	result = defaultRemote.Repository()
	return result, err
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
		return p, err
	}
	p = prompter.New(editor, io.In, io.Out, io.ErrOut)
	return p, err
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
