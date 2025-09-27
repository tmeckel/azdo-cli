package shared

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type FindOptions struct {
	// The ID (number) of the PR to find
	Selector string
	// BaseBranch is the name of the base branch to scope the PR-for-branch lookup to.
	BaseBranch string
	// States lists the possible PR states to scope the PR-for-branch lookup to.
	States []string
}

type progressIndicator interface {
	StartProgressIndicator()
	StopProgressIndicator()
}

type PRFinder interface {
	Find(opts FindOptions) (*git.GitPullRequest, azdo.Repository, error)
}

type finder struct {
	repoCtx  util.RepoContext
	progress progressIndicator
	ctx      context.Context
}

var pullRE = regexp.MustCompile(`^(((?P<Org>[\w-_.\s]+)\/)?((?P<Prj>[\w-_.\s]+)\/)?(?P<Repo>[\w-_.\s]+):)?#?(?P<PrId>\d+)$`)

func parseSelector(selector string) (org string, proj string, repo string, prid int, err error) {
	m := pullRE.FindStringSubmatch(selector)
	if m == nil {
		err = fmt.Errorf("not a valid pull request selector: %q", selector)
		return org, proj, repo, prid, err
	}
	for _, g := range []string{"Org", "Prj", "Repo", "PrId"} {
		gi := pullRE.SubexpIndex(g)
		if gi < 0 || gi > len(m) {
			continue
		}
		switch g {
		case "Org":
			org = m[gi]
		case "Prj":
			proj = m[gi]
		case "Repo":
			repo = m[gi]
		case "PrId":
			id, e := strconv.Atoi(m[gi])
			if e != nil {
				err = fmt.Errorf("PR ID %q is not a number %w", m[gi], e)
				return org, proj, repo, prid, err
			}
			prid = id
		}
	}
	return org, proj, repo, prid, err
}

func NewFinder(ctx util.CmdContext) (f PRFinder, err error) {
	iostrms, err := ctx.IOStreams()
	if err != nil {
		return
	}
	f = &finder{
		repoCtx:  ctx.RepoContext(),
		progress: iostrms,
		ctx:      ctx.Context(),
	}
	return
}

func (f *finder) Find(opts FindOptions) (*git.GitPullRequest, azdo.Repository, error) {
	if f.progress != nil {
		f.progress.StartProgressIndicator()
		defer f.progress.StopProgressIndicator()
	}

	var repo azdo.Repository
	var prID int
	var branchName string

	if opts.Selector == "" {
		if branch, prNumber, err := f.parseCurrentBranch(); err != nil {
			return nil, nil, err
		} else if prNumber > 0 {
			prID = prNumber
		} else {
			branchName = branch
		}
	} else {
		org, prj, rname, id, err := parseSelector(opts.Selector)
		if err != nil {
			return nil, nil, err
		}
		prID = id
		if org != "" {
			// if we have a fully qualified selector we require a new orgnization connection
			f.repoCtx.WithRepo(func() (azdo.Repository, error) {
				return azdo.NewRepositoryWithOrganization(org, prj, rname)
			})
			defer func() {
				// cleanup repo override (will execute leaving function, not current block)
				f.repoCtx.WithRepo(nil)
			}()
		}
	}

	repo, err := f.repoCtx.Repo()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get repo: %w", err)
	}

	gitClient, err := f.repoCtx.GitClient()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get git client: %w", err)
	}

	var pr *git.GitPullRequest
	if branchName != "" {
		gitRepo, err := f.repoCtx.GitRepository()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get git repo: %w", err)
		}
		prList, err := gitClient.GetPullRequests(f.ctx, git.GetPullRequestsArgs{
			RepositoryId: types.ToPtr(gitRepo.Id.String()),
			SearchCriteria: &git.GitPullRequestSearchCriteria{
				SourceRefName: types.ToPtr(fmt.Sprintf("refs/heads/%s", branchName)),
			},
			Top: types.ToPtr(1),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get PR list from git repo: %w", err)
		}
		if prList == nil || len(*prList) == 0 {
			return nil, nil, util.NewNoResultsError("pull request not found")
		}
		pr = &(*prList)[0]
	} else {
		_pr, err := gitClient.GetPullRequestById(f.ctx, git.GetPullRequestByIdArgs{
			PullRequestId: &prID,
			Project:       types.ToPtr(repo.Project()),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get PR by ID: %w", err)
		}
		pr = _pr
		if pr == nil {
			return nil, nil, util.NewNoResultsError("pull request not found")
		}
		if opts.BaseBranch != "" {
			sourceBranch := fmt.Sprintf("refs/heads/%s", opts.BaseBranch)
			if !strings.EqualFold(*pr.SourceRefName, sourceBranch) {
				return nil, nil, util.NewNoResultsError(fmt.Sprintf("pull request %q does not have base branch %q", opts.Selector, opts.BaseBranch))
			}
		}
	}

	if len(opts.States) > 0 && !slices.ContainsFunc(opts.States, func(v string) bool {
		return strings.EqualFold(string(*pr.Status), v)
	}) {
		return nil, nil, util.NewNoResultsError(fmt.Sprintf("pull request %q is not in any of the specified states %v", opts.Selector, opts.States))
	}
	return pr, repo, nil
}

var prHeadRE = regexp.MustCompile(`^refs/pull/(\d+)/head$`)

func (f *finder) parseCurrentBranch() (string, int, error) {
	gitCmd, err := f.repoCtx.GitCommand()
	if err != nil {
		return "", -1, fmt.Errorf("failed to get git command: %w", err)
	}
	prHeadRef, err := gitCmd.CurrentBranch(f.ctx)
	if err != nil {
		return "", -1, fmt.Errorf("failed to get current branch: %w", err)
	}

	branchConfig := gitCmd.ReadBranchConfig(f.ctx, prHeadRef)

	// the branch is configured to merge a special PR head ref
	if m := prHeadRE.FindStringSubmatch(branchConfig.MergeRef); m != nil {
		prNumber, _ := strconv.Atoi(m[1])
		return "", prNumber, nil
	}

	var branchOwner string
	if branchConfig.RemoteURL != nil {
		// the branch merges from a remote specified by URL
		if r, err := azdo.RepositoryFromURL(branchConfig.RemoteURL); err == nil {
			branchOwner = r.FullName()
		}
	} else if branchConfig.RemoteName != "" {
		// the branch merges from a remote specified by name
		rem, _ := f.repoCtx.Remotes()
		if r, err := rem.FindByName(branchConfig.RemoteName); err == nil {
			branchOwner = r.Repository().FullName()
		}
	}

	if branchOwner != "" {
		if strings.HasPrefix(branchConfig.MergeRef, "refs/heads/") {
			prHeadRef = strings.TrimPrefix(branchConfig.MergeRef, "refs/heads/")
		}
		// prepend `OWNER:` if this branch is pushed to a fork
		repo, err := f.repoCtx.Repo()
		if err != nil {
			return "", -1, err
		}
		if !strings.EqualFold(branchOwner, repo.FullName()) {
			prHeadRef = fmt.Sprintf("%s:%s", branchOwner, prHeadRef)
		}
	}

	return prHeadRef, 0, nil
}
