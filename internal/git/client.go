package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/cli/safeexec"
	"github.com/tmeckel/azdo-cli/internal/iostreams"
	"go.uber.org/zap"
)

var remoteRE = regexp.MustCompile(`(.+)\s+(.+)\s+\((push|fetch)\)`)

// This regexp exists to match lines of the following form:
// 6a6872b918c601a0e730710ad8473938a7516d30\u0000title 1\u0000Body 1\u0000\n
// 7a6872b918c601a0e730710ad8473938a7516d31\u0000title 2\u0000Body 2\u0000
//
// This is the format we use when collecting commit information,
// with null bytes as separators. Using null bytes this way allows for us
// to easily maintain newlines that might be in the body.
//
// The ?m modifier is the multi-line modifier, meaning that ^ and $
// match the beginning and end of lines, respectively.
//
// The [\S\s] matches any whitespace or non-whitespace character,
// which is different from .* because it allows for newlines as well.
//
// The ? following .* and [\S\s] is a lazy modifier, meaning that it will
// match as few characters as possible while still satisfying the rest of the regexp.
// This is important because it allows us to match the first null byte after the title and body,
// rather than the last null byte in the entire string.
var commitLogRE = regexp.MustCompile(`(?m)^[0-9a-fA-F]{7,40}\x00.*?\x00[\S\s]*?\x00$`)

type errWithExitCode interface {
	ExitCode() int
}

type GitCommand interface {
	AddRemote(ctx context.Context, name, urlStr string, trackingBranches []string) (*Remote, error)
	AuthenticatedCommand(ctx context.Context, args ...string) (*Command, error)
	CheckoutBranch(ctx context.Context, branch string) error
	CheckoutNewBranch(ctx context.Context, remoteName, branch string) error
	Clone(ctx context.Context, cloneURL string, args []string, mods ...CommandModifier) (string, error)
	Command(ctx context.Context, args ...string) (*Command, error)
	CommitBody(ctx context.Context, sha string) (string, error)
	Commits(ctx context.Context, baseRef, headRef string) ([]*Commit, error)
	CurrentBranch(ctx context.Context) (string, error)
	DeleteLocalBranch(ctx context.Context, branch string) error
	DeleteLocalTag(ctx context.Context, tag string) error
	Fetch(ctx context.Context, remote string, refspec string, mods ...CommandModifier) error
	GetAuthConfig(ctx context.Context) (authConfig []string, err error)
	GetAzDoPath() string
	GetConfig(ctx context.Context, name string) (string, error)
	GetGitPath() string
	GetRepoDir() string
	GitDir(ctx context.Context) (string, error)
	HasLocalBranch(ctx context.Context, branch string) bool
	HasRemoteBranch(ctx context.Context, remote string, branch string) bool
	IsLocalGitRepo(ctx context.Context) (bool, error)
	LastCommit(ctx context.Context) (*Commit, error)
	ParentBranch(ctx context.Context, branch string) (string, error)
	PathFromRoot(ctx context.Context) string
	Pull(ctx context.Context, remote, branch string, mods ...CommandModifier) error
	Push(ctx context.Context, remote string, ref string, mods ...CommandModifier) error
	ReadBranchConfig(ctx context.Context, branch string) BranchConfig
	Remotes(ctx context.Context) (RemoteSet, error)
	// SetAzDoPath(string) error
	SetConfig(ctx context.Context, configItems ...string) (err error)
	// SetGitPath(string) error
	SetRemoteBranches(ctx context.Context, remote string, refspec string) error
	SetRemoteResolution(ctx context.Context, name, resolution string) error
	SetRepoDir(string) error
	ShowRefs(ctx context.Context, refs []string) ([]Ref, error)
	ToplevelDir(ctx context.Context) (string, error)
	TrackingBranchNames(ctx context.Context, prefix string) []string
	UncommittedChangeCount(ctx context.Context) (int, error)
	UnsetRemoteResolution(ctx context.Context, name string) error
	UpdateRemoteURL(ctx context.Context, name, url string) error
}

type client struct {
	AzDoPath string
	RepoDir  string
	GitPath  string
	Stderr   io.Writer
	Stdin    io.Reader
	Stdout   io.Writer

	commandContext commandCtx
	mu             sync.Mutex
}

func NewGitCommand(io *iostreams.IOStreams) (c GitCommand, err error) {
	azdoPath, err := os.Executable()
	if err != nil {
		return c, err
	}
	c = &client{
		AzDoPath: azdoPath,
		Stderr:   io.ErrOut,
		Stdin:    io.In,
		Stdout:   io.Out,
	}
	return c, err
}

func (c *client) GetAzDoPath() string {
	return c.AzDoPath
}

func (c *client) SetAzDoPath(azDoPath string) error {
	c.AzDoPath = azDoPath
	return nil
}

func (c *client) GetRepoDir() string {
	return c.RepoDir
}

func (c *client) SetRepoDir(repoDir string) error {
	c.RepoDir = repoDir
	return nil
}

func (c *client) GetGitPath() string {
	return c.GitPath
}

func (c *client) SetGitPath(gitPath string) error {
	c.GitPath = gitPath
	return nil
}

func (c *client) Copy() GitCommand {
	return &client{
		AzDoPath: c.AzDoPath,
		RepoDir:  c.RepoDir,
		GitPath:  c.GitPath,
		Stderr:   c.Stderr,
		Stdin:    c.Stdin,
		Stdout:   c.Stdout,

		commandContext: c.commandContext,
	}
}

func (c *client) Command(ctx context.Context, args ...string) (*Command, error) {
	if c.RepoDir != "" {
		args = append([]string{"-C", c.RepoDir}, args...)
	}
	commandContext := exec.CommandContext
	if c.commandContext != nil {
		commandContext = c.commandContext
	}
	var err error
	c.mu.Lock()
	if c.GitPath == "" {
		c.GitPath, err = resolveGitPath()
	}
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	cmd := commandContext(ctx, c.GitPath, args...)
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	return &Command{cmd}, nil
}

func (c *client) GetAuthConfig(ctx context.Context) (authConfig []string, err error) {
	authConfig = []string{"credential.useHttpPath", "true"}
	if c.AzDoPath == "" {
		// Assumes that azdo is in PATH.
		c.AzDoPath = "azdo"
	}
	credHelper := fmt.Sprintf("!%q auth git-credential", c.AzDoPath)
	authConfig = append(authConfig, "credential.helper", credHelper)
	return authConfig, err
}

// AuthenticatedCommand is a wrapper around Command that included configuration to use azdo
// as the credential helper for git.
func (c *client) AuthenticatedCommand(ctx context.Context, args ...string) (*Command, error) {
	authConfig, err := c.GetAuthConfig(ctx)
	if err != nil {
		return nil, err
	}
	authArgs := []string{}
	n := 0
	for n < len(authConfig) {
		authArgs = append(authArgs, "-c", fmt.Sprintf("%s=%s", authConfig[n], authConfig[n+1]))
		n += 2
	}
	args = append(authArgs, args...)
	return c.Command(ctx, args...)
}

func (c *client) Remotes(ctx context.Context) (RemoteSet, error) {
	remoteArgs := []string{"remote", "-v"}
	remoteCmd, err := c.Command(ctx, remoteArgs...)
	if err != nil {
		return nil, err
	}
	remoteOut, remoteErr := remoteCmd.Output()
	if remoteErr != nil {
		return nil, remoteErr
	}

	configArgs := []string{"config", "--get-regexp", `^remote\..*\.azdo-resolved$`}
	configCmd, err := c.Command(ctx, configArgs...)
	if err != nil {
		return nil, err
	}
	configOut, configErr := configCmd.Output()
	if configErr != nil {
		// Ignore exit code 1 as it means there are no resolved remotes.
		var gitErr *GitError
		if ok := errors.As(configErr, &gitErr); ok && gitErr.ExitCode != 1 {
			return nil, gitErr
		}
	}

	remotes := parseRemotes(outputLines(remoteOut))

	zap.L().Sugar().Debugf("Found remotes %+v ", remotes)

	populateResolvedRemotes(remotes, outputLines(configOut))
	return remotes, nil
}

func (c *client) UpdateRemoteURL(ctx context.Context, name, url string) error {
	args := []string{"remote", "set-url", name, url}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) SetRemoteResolution(ctx context.Context, name, resolution string) error {
	args := []string{"config", "--add", fmt.Sprintf("remote.%s.azdo-resolved", name), resolution}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

// CurrentBranch reads the checked-out branch for the git repository.
func (c *client) CurrentBranch(ctx context.Context) (string, error) {
	args := []string{"symbolic-ref", "--quiet", "HEAD"}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		var gitErr *GitError
		if ok := errors.As(err, &gitErr); ok && len(gitErr.Stderr) == 0 {
			gitErr.err = ErrNotOnAnyBranch
			gitErr.Stderr = "not on any branch"
			return "", gitErr
		}
		return "", err
	}
	branch := firstLine(out)
	return strings.TrimPrefix(branch, "refs/heads/"), nil
}

func (c *client) ParentBranch(ctx context.Context, branch string) (string, error) {
	args := []string{"show-branch", "-a", branch}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return "", err
	}

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	currentBranch := branch
	re := regexp.MustCompile(`\[(.*?)\]`)

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "*") && !strings.Contains(line, currentBranch) {
			// Extract branch name from the line
			// This is a simplified extraction, might need adjustment based on actual output format
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				// Remove any ~1 or ^1 suffixes
				parentBranch := strings.Split(matches[1], "~")[0]
				parentBranch = strings.Split(parentBranch, "^")[0]
				return parentBranch, nil
			}
		}
	}

	return "", fmt.Errorf("no parent branch found for %s", branch)
}

// ShowRefs resolves fully-qualified refs to commit hashes.
func (c *client) ShowRefs(ctx context.Context, refs []string) ([]Ref, error) {
	args := append([]string{"show-ref", "--verify", "--"}, refs...)
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return nil, err
	}
	// This functionality relies on parsing output from the git command despite
	// an error status being returned from git.
	out, err := cmd.Output()
	var verified []Ref
	for _, line := range outputLines(out) {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		verified = append(verified, Ref{
			Hash: parts[0],
			Name: parts[1],
		})
	}
	if err != nil {
		return verified, err
	}
	return verified, err
}

func (c *client) SetConfig(ctx context.Context, configItems ...string) (err error) {
	if len(configItems) > 0 {
		var cmd *Command
		if len(configItems)%2 > 0 {
			return fmt.Errorf("configuration parameters must by symmetric")
		}
		n := 0
		for n < len(configItems) {
			args := append([]string{"config"}, configItems[n], configItems[n+1])
			cmd, err = c.Command(ctx, args...)
			if err != nil {
				return err
			}
			err = cmd.Run()
			if err != nil {
				err = fmt.Errorf("failed to set config item %s to value %s: %w", configItems[n], configItems[n+1], err)
				return err
			}
			n += 2
		}
	}
	return err
}

func (c *client) GetConfig(ctx context.Context, name string) (string, error) {
	args := []string{"config", name}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return "", err
	}
	out, err := cmd.Output()
	if err != nil {
		var gitErr *GitError
		if ok := errors.As(err, &gitErr); ok && gitErr.ExitCode == 1 {
			gitErr.Stderr = fmt.Sprintf("unknown config key %s", name)
			return "", gitErr
		}
		return "", err
	}
	return firstLine(out), nil
}

func (c *client) UncommittedChangeCount(ctx context.Context) (int, error) {
	args := []string{"status", "--porcelain"}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return 0, err
	}
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(out), "\n")
	count := 0
	for _, l := range lines {
		if l != "" {
			count++
		}
	}
	return count, nil
}

func (c *client) Commits(ctx context.Context, baseRef, headRef string) ([]*Commit, error) {
	// The formatting directive %x00 indicates that git should include the null byte as a separator.
	// We use this because it is not a valid character to include in a commit message. Previously,
	// commas were used here but when we Split on them, we would get incorrect results if commit titles
	// happened to contain them.
	// https://git-scm.com/docs/pretty-formats#Documentation/pretty-formats.txt-emx00em
	args := []string{"-c", "log.ShowSignature=false", "log", "--pretty=format:%H%x00%s%x00%b%x00", "--cherry", fmt.Sprintf("%s...%s", baseRef, headRef)}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	commits := []*Commit{}
	commitLogs := commitLogRE.FindAllString(string(out), -1)
	for _, commitLog := range commitLogs {
		//  Each line looks like this:
		//  6a6872b918c601a0e730710ad8473938a7516d30\u0000title 1\u0000Body 1\u0000\n

		//  Or with an optional body:
		//  6a6872b918c601a0e730710ad8473938a7516d30\u0000title 1\u0000\u0000\n

		//  Therefore after splitting we will have:
		//  ["6a6872b918c601a0e730710ad8473938a7516d30", "title 1", "Body 1", ""]

		//  Or with an optional body:
		//  ["6a6872b918c601a0e730710ad8473938a7516d30", "title 1", "", ""]
		commitLogParts := strings.Split(commitLog, "\u0000")
		commits = append(commits, &Commit{
			Sha:   commitLogParts[0],
			Title: commitLogParts[1],
			Body:  commitLogParts[2],
		})
	}

	if len(commits) == 0 {
		return nil, fmt.Errorf("could not find any commits between %s and %s", baseRef, headRef)
	}

	return commits, nil
}

func (c *client) LastCommit(ctx context.Context) (*Commit, error) {
	output, err := c.lookupCommit(ctx, "HEAD", "%H,%s")
	if err != nil {
		return nil, err
	}
	idx := bytes.IndexByte(output, ',')
	return &Commit{
		Sha:   string(output[0:idx]),
		Title: strings.TrimSpace(string(output[idx+1:])),
	}, nil
}

func (c *client) CommitBody(ctx context.Context, sha string) (string, error) {
	output, err := c.lookupCommit(ctx, sha, "%b")
	return string(output), err
}

func (c *client) lookupCommit(ctx context.Context, sha, format string) ([]byte, error) {
	args := []string{"-c", "log.ShowSignature=false", "show", "-s", "--pretty=format:" + format, sha}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReadBranchConfig parses the `branch.BRANCH.(remote|merge)` part of git config.
func (c *client) ReadBranchConfig(ctx context.Context, branch string) (cfg BranchConfig) {
	prefix := regexp.QuoteMeta(fmt.Sprintf("branch.%s.", branch))
	args := []string{"config", "--get-regexp", fmt.Sprintf("^%s(remote|merge)$", prefix)}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return cfg
	}
	out, err := cmd.Output()
	if err != nil {
		return cfg
	}
	for _, line := range outputLines(out) {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}
		keys := strings.Split(parts[0], ".")
		switch keys[len(keys)-1] {
		case "remote":
			if strings.Contains(parts[1], ":") {
				u, err := ParseURL(parts[1])
				if err != nil {
					continue
				}
				cfg.RemoteURL = u
			} else if !isFilesystemPath(parts[1]) {
				cfg.RemoteName = parts[1]
			}
		case "merge":
			cfg.MergeRef = parts[1]
		}
	}
	return cfg
}

func (c *client) DeleteLocalTag(ctx context.Context, tag string) error {
	args := []string{"tag", "-d", tag}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) DeleteLocalBranch(ctx context.Context, branch string) error {
	args := []string{"branch", "-D", branch}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) CheckoutBranch(ctx context.Context, branch string) error {
	args := []string{"checkout", branch}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) CheckoutNewBranch(ctx context.Context, remoteName, branch string) error {
	track := fmt.Sprintf("%s/%s", remoteName, branch)
	args := []string{"checkout", "-b", branch, "--track", track}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) HasLocalBranch(ctx context.Context, branch string) bool {
	if !strings.HasPrefix(branch, "refs/heads/") {
		branch = "refs/heads/" + branch
	}
	_, err := c.revParse(ctx, "--verify", branch)
	return err == nil
}

func (c *client) HasRemoteBranch(ctx context.Context, remote string, branch string) bool {
	branch = fmt.Sprintf("%s/%s", remote, strings.TrimPrefix(branch, "refs/heads/"))
	_, err := c.revParse(ctx, "--verify", branch)
	return err == nil
}

func (c *client) TrackingBranchNames(ctx context.Context, prefix string) []string {
	args := []string{"branch", "-r", "--format", "%(refname:strip=3)"}
	if prefix != "" {
		args = append(args, "--list", fmt.Sprintf("*/%s*", escapeGlob(prefix)))
	}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return nil
	}
	output, err := cmd.Output()
	if err != nil {
		return nil
	}
	return strings.Split(string(output), "\n")
}

// ToplevelDir returns the top-level directory path of the current repository.
func (c *client) ToplevelDir(ctx context.Context) (string, error) {
	out, err := c.revParse(ctx, "--show-toplevel")
	if err != nil {
		return "", err
	}
	return firstLine(out), nil
}

func (c *client) GitDir(ctx context.Context) (string, error) {
	out, err := c.revParse(ctx, "--git-dir")
	if err != nil {
		return "", err
	}
	return firstLine(out), nil
}

// Show current directory relative to the top-level directory of repository.
func (c *client) PathFromRoot(ctx context.Context) string {
	out, err := c.revParse(ctx, "--show-prefix")
	if err != nil {
		return ""
	}
	if path := firstLine(out); path != "" {
		return path[:len(path)-1]
	}
	return ""
}

func (c *client) revParse(ctx context.Context, args ...string) ([]byte, error) {
	args = append([]string{"rev-parse"}, args...)
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return nil, err
	}
	return cmd.Output()
}

func (c *client) IsLocalGitRepo(ctx context.Context) (bool, error) {
	_, err := c.GitDir(ctx)
	if err != nil {
		var execError errWithExitCode
		if errors.As(err, &execError) && execError.ExitCode() == 128 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *client) UnsetRemoteResolution(ctx context.Context, name string) error {
	args := []string{"config", "--unset", fmt.Sprintf("remote.%s.azdo-resolved", name)}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) SetRemoteBranches(ctx context.Context, remote string, refspec string) error {
	args := []string{"remote", "set-branches", remote, refspec}
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return err
	}
	_, err = cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) AddRemote(ctx context.Context, name, urlStr string, trackingBranches []string) (*Remote, error) {
	args := []string{"remote", "add"}
	for _, branch := range trackingBranches {
		args = append(args, "-t", branch)
	}
	args = append(args, name, urlStr)
	cmd, err := c.Command(ctx, args...)
	if err != nil {
		return nil, err
	}
	if _, err := cmd.Output(); err != nil {
		return nil, err
	}
	var urlParsed *url.URL
	if strings.HasPrefix(urlStr, "http") {
		urlParsed, err = url.Parse(urlStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL %q: %w", urlStr, err)
		}
	} else {
		urlParsed, err = ParseURL(urlStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse URL %q: %w", urlStr, err)
		}
	}
	remote := &Remote{
		Name:     name,
		FetchURL: urlParsed,
		PushURL:  urlParsed,
	}
	return remote, nil
}

// Below are commands that make network calls and need authentication credentials supplied from gh.

func (c *client) Fetch(ctx context.Context, remote string, refspec string, mods ...CommandModifier) error {
	args := []string{"fetch", remote}
	if refspec != "" {
		args = append(args, refspec)
	}
	cmd, err := c.AuthenticatedCommand(ctx, args...)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		mod(cmd)
	}
	return cmd.Run()
}

func (c *client) Pull(ctx context.Context, remote, branch string, mods ...CommandModifier) error {
	args := []string{"pull", "--ff-only"}
	if remote != "" && branch != "" {
		args = append(args, remote, branch)
	}
	cmd, err := c.AuthenticatedCommand(ctx, args...)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		mod(cmd)
	}
	return cmd.Run()
}

func (c *client) Push(ctx context.Context, remote string, ref string, mods ...CommandModifier) error {
	args := []string{"push", "--set-upstream", remote, ref}
	cmd, err := c.AuthenticatedCommand(ctx, args...)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		mod(cmd)
	}
	return cmd.Run()
}

func (c *client) Clone(ctx context.Context, cloneURL string, args []string, mods ...CommandModifier) (string, error) {
	cloneArgs, target := parseCloneArgs(args)
	cloneArgs = append(cloneArgs, cloneURL)
	// If the args contain an explicit target, pass it to clone otherwise,
	// parse the URL to determine where git cloned it to so we can return it.
	if target != "" {
		cloneArgs = append(cloneArgs, target)
	} else {
		target = path.Base(strings.TrimSuffix(cloneURL, ".git"))
	}
	cloneArgs = append([]string{"clone"}, cloneArgs...)
	cmd, err := c.AuthenticatedCommand(ctx, cloneArgs...)
	if err != nil {
		return "", err
	}
	for _, mod := range mods {
		mod(cmd)
	}
	err = cmd.Run()
	if err != nil {
		return "", err
	}
	return target, nil
}

func resolveGitPath() (string, error) {
	path, err := safeexec.LookPath("git")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			programName := "git"
			if runtime.GOOS == "windows" {
				programName = "Git for Windows"
			}
			return "", &NotInstalledError{
				message: fmt.Sprintf("unable to find git executable in PATH; please install %s before retrying", programName),
				err:     err,
			}
		}
		return "", err
	}
	return path, nil
}

func isFilesystemPath(p string) bool {
	return p == "." || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "/")
}

func outputLines(output []byte) []string {
	lines := strings.TrimSuffix(string(output), "\n")
	return strings.Split(lines, "\n")
}

func firstLine(output []byte) string {
	if i := bytes.IndexAny(output, "\n"); i >= 0 {
		return string(output)[0:i]
	}
	return string(output)
}

func parseCloneArgs(extraArgs []string) (args []string, target string) {
	args = extraArgs
	if len(args) > 0 {
		if !strings.HasPrefix(args[0], "-") {
			target, args = args[0], args[1:]
		}
	}
	return args, target
}

func parseRemotes(remotesStr []string) RemoteSet {
	zap.L().Sugar().Debugf("Parsing remotes: %+v", remotesStr)
	remotes := RemoteSet{}
	for _, r := range remotesStr {
		match := remoteRE.FindStringSubmatch(r)
		if match == nil {
			zap.L().Sugar().Debugf("Skipping unparsable remote line: %q", r)
			continue
		}
		name := strings.TrimSpace(match[1])
		urlStr := strings.TrimSpace(match[2])
		urlType := strings.TrimSpace(match[3])

		zap.L().Sugar().Debugf("Found remote name=%q url=%q type=%q", name, urlStr, urlType)
		url, err := ParseURL(urlStr)
		if err != nil {
			continue
		}

		var rem *Remote
		if len(remotes) > 0 {
			/**
			 * because we are parsing the output of `git remote -v` which groups remotes together
			 * we can optimize the search by first checking the last remote we added
			 * and seeing if it matches the current remote name
			 * before searching through the entire list of remotes
			 **/
			zap.L().Sugar().Debugf("Checking last remote %q against last remote %q", remotes[len(remotes)-1].Name, name)
			rem = remotes[len(remotes)-1]
			if name != rem.Name {
				rem = nil
			}
		}
		if rem == nil {
			zap.L().Sugar().Debugf("Adding new remote %q", name)
			rem = &Remote{Name: name}
			remotes = append(remotes, rem)
		}

		switch urlType {
		case "fetch":
			zap.L().Sugar().Debugf("Setting fetch URL for remote %q to %q", name, urlStr)
			rem.FetchURL = url
		case "push":
			zap.L().Sugar().Debugf("Setting push URL for remote %q to %q", name, urlStr)
			rem.PushURL = url
		}
	}
	return remotes
}

func populateResolvedRemotes(remotes RemoteSet, resolved []string) {
	for _, l := range resolved {
		zap.L().Sugar().Debugf("Parsing resolved remote line: %q", l)
		parts := strings.SplitN(l, " ", 2)
		if len(parts) < 2 {
			continue
		}
		rp := strings.SplitN(parts[0], ".", 3)
		if len(rp) < 2 {
			continue
		}
		name := rp[1]
		for _, r := range remotes {
			if r.Name == name {
				zap.L().Sugar().Debugf("Setting resolved URL for remote %q to %q", name, parts[1])
				r.Resolved = parts[1]
				break
			}
		}
	}
}

var globReplacer = strings.NewReplacer(
	"*", `\*`,
	"?", `\?`,
	"[", `\[`,
	"]", `\]`,
	"{", `\{`,
	"}", `\}`,
)

func escapeGlob(p string) string {
	return globReplacer.Replace(p)
}
