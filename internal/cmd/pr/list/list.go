package list

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/azdo"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/printer"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type listOptions struct {
	limitResults int
	state        string
	mergeState   string
	baseBranch   string
	headBranch   string
	labels       []string
	author       string
	reviewer     string
	draft        *bool
	format       string
	exporter     util.Exporter
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list [[organization/]project/repository]",
		Short: "List pull requests in a repository or a project",
		Long: heredoc.Doc(`
			List pull requests in a Azure DevOps repository or project.
		`),
		Example: heredoc.Doc(`
			List open PRs authored by you
			$ azdo pr list --author "@me"

			List only PRs with all of the given labels
			$ azdo pr list --label bug --label "priority 1"

			Find a PRs that are completed
			$ azdo pr list --state completed

			List PRs using a template
			$ azdo pr list --json pullRequestId,title --template '{{range.}}{{printf "#%.0f - %s\n" .pullRequestId .title}}{{end}}'

			List PRs using a JQ filter and a template to render the result
			$ azdo pr list \
			     --json pullRequestId,title,isDraft,labels \
			     --jq '.[] | select(.title | contains("dependency"))' \
			     -t '{{range.}}{{printf "#%.0f - %s\n" .pullRequestId .title}}{{end}}'
    	`),
		Aliases: []string{"ls"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				ctx.RepoContext().WithRepo(func() (azdo.Repository, error) {
					return azdo.RepositoryFromName(args[0])
				})
			}
			if opts.limitResults < 1 {
				return util.FlagErrorf("invalid value for --limit: %v", opts.limitResults)
			}

			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().IntVarP(&opts.limitResults, "limit", "L", 30, "Maximum number of items to fetch")
	util.StringEnumFlag(cmd, &opts.state, "state", "s", "active", []string{"abandoned", "active", "all", "completed"}, "Filter by state")
	util.StringEnumFlag(cmd, &opts.mergeState, "mergestate", "m", "", []string{"succeeded", "conflicts"}, "Filter by merge state")
	cmd.Flags().StringVarP(&opts.baseBranch, "base", "B", "", "Filter by base branch")
	cmd.Flags().StringVarP(&opts.headBranch, "head", "H", "", "Filter by head branch")
	cmd.Flags().StringSliceVarP(&opts.labels, "label", "l", nil, "Filter by label")
	cmd.Flags().StringVarP(&opts.author, "author", "a", "", "Filter by author")
	cmd.Flags().StringVarP(&opts.reviewer, "reviewer", "r", "", "Filter by reviewer")
	util.StringEnumFlag(cmd, &opts.format, "format", "f", "table", []string{"json"}, "Output format")
	util.NilBoolFlag(cmd, &opts.draft, "draft", "d", "Filter by draft state")
	util.AddJSONFlags(cmd, &opts.exporter, shared.PullRequestFields)

	gitClient, err := ctx.RepoContext().GitCommand()
	if err != nil {
		panic(fmt.Sprintf("failed to get Git client: %v", err))
	}
	_ = util.RegisterBranchCompletionFlags(gitClient, cmd, "base", "head")
	return cmd
}

func runCmd(ctx util.CmdContext, opts *listOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	iostreams.StartProgressIndicator()
	defer iostreams.StopProgressIndicator()

	repo, err := ctx.RepoContext().Repo()
	if err != nil {
		return util.FlagErrorf("unable to get current repository: %w", err)
	}

	conn, err := ctx.ConnectionFactory().Connection(repo.Organization())
	if err != nil {
		return
	}

	repoClient, err := git.NewClient(ctx.Context(), conn)
	if err != nil {
		return err
	}

	gitRepo, err := repo.GitRepository(ctx.Context(), repoClient)
	if err != nil {
		return err
	}
	searchCriteria := &git.GitPullRequestSearchCriteria{
		Status: (*git.PullRequestStatus)(&opts.state),
	}

	if opts.baseBranch != "" {
		searchCriteria.TargetRefName = types.ToPtr(fmt.Sprintf("refs/heads/%s", opts.baseBranch))
	}
	if opts.headBranch != "" {
		searchCriteria.SourceRefName = types.ToPtr(fmt.Sprintf("refs/heads/%s", opts.headBranch))
	}

	if opts.author != "" {
		if strings.EqualFold(opts.author, "@me") {
			selfID, err := shared.GetSelfID(ctx.Context(), conn)
			if err != nil {
				return err
			}
			searchCriteria.CreatorId = &selfID
		} else {
			subjectID, err := shared.GetSubjectID(ctx.Context(), conn, opts.author)
			if err != nil {
				return err
			}
			searchCriteria.CreatorId = &subjectID
		}
	}
	if opts.reviewer != "" {
		if strings.EqualFold(opts.reviewer, "@me") {
			selfID, err := shared.GetSelfID(ctx.Context(), conn)
			if err != nil {
				return err
			}
			searchCriteria.ReviewerId = &selfID
		} else {
			subjectID, err := shared.GetSubjectID(ctx.Context(), conn, opts.reviewer)
			if err != nil {
				return err
			}
			searchCriteria.ReviewerId = &subjectID
		}
	}

	var limit *int
	if opts.limitResults > 0 {
		limit = &opts.limitResults
	}
	prList, err := repoClient.GetPullRequests(ctx.Context(), git.GetPullRequestsArgs{
		RepositoryId:   types.ToPtr(gitRepo.Id.String()),
		SearchCriteria: searchCriteria,
		Top:            limit,
	})
	if err != nil {
		return err
	}

	if prList == nil || len(*prList) == 0 {
		return util.NewNoResultsError(fmt.Sprintf("No Pull Requests found for repository %s not found in project %s at organization %s", repo.Name(), repo.Project(), repo.Organization()))
	}

	filters := []func(value git.GitPullRequest, index int) (bool, error){}
	if opts.draft != nil && *opts.draft || len(opts.labels) > 0 {
		filters = append(filters, func(pr git.GitPullRequest, index int) (bool, error) {
			return pr.IsDraft != nil && *pr.IsDraft, nil
		})
	}
	if opts.mergeState != "" {
		filters = append(filters, func(pr git.GitPullRequest, index int) (bool, error) {
			return pr.MergeStatus != nil && opts.mergeState == string(*pr.MergeStatus), nil
		})
	}
	if len(opts.labels) > 0 {
		filters = append(filters, func(pr git.GitPullRequest, index int) (bool, error) {
			return pr.Labels != nil && func() bool {
				for _, l := range *pr.Labels {
					if !slices.Contains(opts.labels, *l.Name) { // TODO: do we have to check for l.Name is nil?
						return false
					}
				}
				return true
			}(), nil
		})
	}

	filteredPrList, err := types.FilterSlice[git.GitPullRequest](*prList, filters...)
	if err != nil {
		return err
	}
	if len(filteredPrList) == 0 {
		return util.NewNoResultsError(fmt.Sprintf("No Pull Requests found for repository %s in project %s at organization %s using specified filters", repo.Name(), repo.Project(), repo.Organization()))
	}
	prList = &filteredPrList

	iostreams.StopProgressIndicator()

	if opts.exporter != nil {
		iostreams, err := ctx.IOStreams()
		if err != nil {
			return err
		}
		return opts.exporter.Write(iostreams, *prList)
	}

	tp, err := ctx.Printer(opts.format)
	if err != nil {
		return
	}

	tp.AddColumns("ID", "Title", "Branch", "Author", "State", "IsDraft", "MergeStatus")
	for _, pr := range *prList {
		tp.AddField(strconv.Itoa(*pr.PullRequestId))
		tp.AddField(*pr.Title, printer.WithTruncate(nil))
		tp.AddField(strings.TrimPrefix(*pr.SourceRefName, "refs/heads/"))
		tp.AddField(fmt.Sprintf("%s (%s)", *pr.CreatedBy.DisplayName, *pr.CreatedBy.UniqueName))
		tp.AddField(string(*pr.Status))
		tp.AddField(strconv.FormatBool(*pr.IsDraft))
		if pr.MergeStatus != nil {
			tp.AddField(string(*pr.MergeStatus))
		} else {
			tp.AddField("unknown")
		}
		tp.EndRow()
	}
	return tp.Render()
}
