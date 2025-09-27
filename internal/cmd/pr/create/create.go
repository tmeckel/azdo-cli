package create

import (
	"context"
	_ "embed"
	"fmt"
	"reflect"
	"strings"

	"github.com/MakeNowJust/heredoc"
	azgit "github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
	"github.com/tmeckel/azdo-cli/internal/text"
	"github.com/tmeckel/azdo-cli/internal/types"
)

type createOptions struct {
	autofill    bool
	fillVerbose bool
	fillFirst   bool
	editorMode  bool
	useTemplate bool
	recoverFile string

	isDraft bool

	title       string
	description string

	baseBranch string
	headBranch string

	requiredReviewer []string
	optionalReviewer []string

	dryRun bool
}

//go:embed create_dry_run.tpl
var dryRunTpl string

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &createOptions{}

	var descriptionFile string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request",
		Long: heredoc.Docf(`
			Create a pull request on Azure DevOps.

			When the current branch isn't fully pushed to a git remote, a prompt will ask where
			to push the branch and offer an option to fork the base repository. Use %[1]s--head%[1]s to
			explicitly skip any forking or pushing behavior.

			A prompt will also ask for the title and the body of the pull request. Use %[1]s--title%[1]s and
			%[1]s--body%[1]s to skip this, or use %[1]s--fill%[1]s to autofill these values from git commits.
			It's important to notice that if the %[1]s--title%[1]s and/or %[1]s--body%[1]s are also provided
			alongside %[1]s--fill%[1]s, the values specified by %[1]s--title%[1]s and/or %[1]s--body%[1]s will
			take precedence and overwrite any autofilled content.

			Link an issue to the pull request by referencing the issue in the body of the pull
			request. If the body text mentions %[1]sFixes #123%[1]s or %[1]sCloses #123%[1]s, the referenced issue
			will automatically get closed when the pull request gets merged.

			By default, users with write access to the base repository can push new commits to the
			head branch of the pull request. Disable this with %[1]s--no-maintainer-edit%[1]s.

			Adding a pull request to projects requires authorization with the %[1]sproject%[1]s scope.
			To authorize, run %[1]sgh auth refresh -s project%[1]s.
		`, "`"),
		Example: heredoc.Doc(`
			$ azdo pr create --title "The bug is fixed" --description "Everything works again"
			$ azdo pr create --reviewer monalisa,hubot  --reviewer myorg/team-name
			$ azdo pr create --base develop --head monalisa:feature
			$ azdo pr create --use-template
		`),
		Args:    util.NoArgsQuoteReminder,
		Aliases: []string{"new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			iostreams, err := ctx.IOStreams()
			if err != nil {
				return util.FlagErrorf("error getting io streams: %w", err)
			}

			if err := util.MutuallyExclusive(
				"`--recover` only supported when running interactively",
				!iostreams.CanPrompt(),
				opts.recoverFile != "",
			); err != nil {
				return err //nolint:error,wrapcheck
			}

			if err := util.MutuallyExclusive(
				"`--fill` is not supported with `--fill-first`",
				opts.autofill,
				opts.fillFirst,
			); err != nil {
				return err //nolint:error,wrapcheck
			}

			if err := util.MutuallyExclusive(
				"`--fill-verbose` is not supported with `--fill-first`",
				opts.fillVerbose,
				opts.fillFirst,
			); err != nil {
				return err //nolint:error,wrapcheck
			}

			if err := util.MutuallyExclusive(
				"`--fill-verbose` is not supported with `--fill`",
				opts.fillVerbose,
				opts.autofill,
			); err != nil {
				return err //nolint:error,wrapcheck
			}

			if err := util.MutuallyExclusive(
				"`--description` is not supported with `--description-file`",
				opts.description != "",
				descriptionFile != "",
			); err != nil {
				return err //nolint:error,wrapcheck
			}

			if err := util.MutuallyExclusive(
				"`--use-template` is not supported with `--description-file` or `--description`",
				opts.useTemplate,
				opts.description != "" || descriptionFile != "",
			); err != nil {
				return err //nolint:error,wrapcheck
			}

			opts.editorMode, err = shared.InitEditorMode(ctx, opts.editorMode, false, iostreams.CanPrompt())
			if err != nil {
				return err
			}

			if descriptionFile != "" {
				b, err := util.ReadFile(descriptionFile, iostreams.In)
				if err != nil {
					return err
				}
				opts.description = string(b)
			}

			if opts.useTemplate {
				tm, err := shared.NewTemplateManager(ctx)
				if err != nil {
					return err
				}
				r, err := ctx.RepoContext().Repo()
				if err != nil {
					return fmt.Errorf("failed to get repository from context: %w", err)
				}
				t, err := tm.GetTemplate(ctx.Context(), r, opts.baseBranch)
				if err != nil {
					return fmt.Errorf("failed to get pull request template from repository: %w", err)
				}
				if t == nil {
					return fmt.Errorf("no pull request template found in repository")
				}
				opts.description = string(t.Body())
			}

			if !iostreams.CanPrompt() && !(opts.fillVerbose || opts.autofill || opts.fillFirst) && (opts.title == "" || opts.description == "") {
				return util.FlagErrorf("must provide `--title` and `--description` (`--description-file`) or `--fill` or `fill-first` or `--fillverbose` when not running interactively")
			}

			return runCmd(ctx, opts)
		},
	}

	fl := cmd.Flags()
	fl.BoolVarP(&opts.isDraft, "draft", "d", false, "Mark pull request as a draft")
	fl.StringVarP(&opts.title, "title", "t", "", "Title for the pull request")
	fl.StringVarP(&opts.description, "description", "D", "", "Description for the pull request")
	fl.StringVarP(&descriptionFile, "description-file", "F", "", "Read description text from `file` (use \"-\" to read from standard input)")
	fl.StringVarP(&opts.baseBranch, "base", "B", "", "The `branch` into which you want your code merged")
	fl.StringVarP(&opts.headBranch, "head", "H", "", "The `branch` that contains commits for your pull request (default [current branch])")
	fl.BoolVarP(&opts.fillVerbose, "fill-verbose", "", false, "Use commits msg+body for description")
	fl.BoolVarP(&opts.autofill, "fill", "f", false, "Use commit info for title and body")
	fl.BoolVar(&opts.fillFirst, "fill-first", false, "Use first commit info for title and body")
	fl.BoolVar(&opts.useTemplate, "use-template", false, "Use a pull request template for the description of the new pull request. The command will fail if no template is found")
	fl.StringSliceVarP(&opts.requiredReviewer, "required-reviewer", "r", nil, "Required reviewers (comma-separated)")
	fl.StringSliceVarP(&opts.optionalReviewer, "optional-reviewer", "o", nil, "Optional reviewers (comma-separated)")
	fl.StringVar(&opts.recoverFile, "recover", "", "Recover input from a failed run of create")
	fl.BoolVar(&opts.dryRun, "dry-run", false, "Print details instead of creating the PR. May still push git changes.")

	gitClient, err := ctx.RepoContext().GitCommand()
	if err != nil {
		panic(fmt.Sprintf("failed to get Git client: %v", err))
	}
	_ = util.RegisterBranchCompletionFlags(gitClient, cmd, "base", "head")

	return cmd
}

func runCmd(ctx util.CmdContext, opts *createOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}

	repo, err := ctx.RepoContext().GitRepository()
	if err != nil {
		return fmt.Errorf("failed to get remote repository: %w", err)
	}

	remote, err := ctx.RepoContext().Remote(repo)
	if err != nil {
		return fmt.Errorf("failed to find local remote for repository: %w", err)
	}
	if opts.baseBranch == "" {
		if repo.DefaultBranch == nil {
			return fmt.Errorf("repository does not specify a default branch. Specify the base branch using --base or -B")
		}
		opts.baseBranch = normalizeBranch(*repo.DefaultBranch)
	} else {
		// Normalize provided base branch to allow inputs like "refs/heads/<name>"
		opts.baseBranch = normalizeBranch(opts.baseBranch)
	}

	if opts.headBranch != "" {
		opts.headBranch = normalizeBranch(opts.headBranch)
	}

	// Prequisites
	// 1. Is the current branch the same as the base branch?
	// 2. Is the current branch the same as the head branch?
	// 3. Does the head branch exist?
	// 4. Is the current branch pushed to a remote?
	// 5. Does a PR already exists for the head branch into the base branch?

	gitCmd, err := ctx.RepoContext().GitCommand()
	if err != nil {
		return fmt.Errorf("failed to get git client: %w", err)
	}

	currentBranch, err := gitCmd.CurrentBranch(ctx.Context())
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == opts.baseBranch {
		return fmt.Errorf("current branch '%s' is the same as base branch. Cannot create PR from a branch to itself", currentBranch)
	}

	if opts.headBranch == "" {
		opts.headBranch = currentBranch
	}

	if opts.headBranch == opts.baseBranch {
		return fmt.Errorf("head branch '%s' is the same as base branch. Cannot create PR from a branch to itself", opts.headBranch)
	}

	if !gitCmd.HasLocalBranch(ctx.Context(), opts.headBranch) {
		return fmt.Errorf("head branch '%s' does not exist", opts.headBranch)
	}

	if currentBranch == opts.headBranch {
		if ucc, err := gitCmd.UncommittedChangeCount(context.Background()); err == nil && ucc > 0 {
			fmt.Fprintf(iostreams.ErrOut, "Warning: current branch contains %s\n", text.Pluralize(ucc, "uncommitted change"))
		}
	}

	// Ensure head branch is pushed to remote if missing there
	if !gitCmd.HasRemoteBranch(ctx.Context(), remote.Name, opts.headBranch) {
		if err = gitCmd.Push(ctx.Context(), remote.Name, opts.headBranch); err != nil {
			return fmt.Errorf("failed to push head branch '%s' to remote: %w", opts.headBranch, err)
		}
	}

	if opts.autofill || opts.fillFirst || opts.fillVerbose {
		commits, err := gitCmd.Commits(ctx.Context(), opts.baseBranch, opts.headBranch)
		if err != nil {
			return fmt.Errorf("failed to get commits: %w", err)
		}
		if len(commits) == 0 {
			return fmt.Errorf("no commits found on branch. Unable to auto fill")
		}
		if opts.fillFirst {
			if opts.title == "" {
				opts.title = commits[0].Title
			}
			if opts.description == "" {
				opts.description = commits[0].Body
			}
		} else {
			if opts.title == "" {
				opts.title = commits[len(commits)-1].Title
			}
			if opts.description == "" {
				var sb strings.Builder
				for _, c := range commits {
					if opts.fillVerbose {
						sb.WriteString(fmt.Sprintf("### %s\n%s\n", c.Title, c.Body))
					} else {
						sb.WriteString(fmt.Sprintf("* %s", c.Title))
					}
				}
				opts.description = sb.String()
			}
		}
	}

	// Existing PR check via REST using explicit source/target refs
	restGitClient, err := ctx.RepoContext().GitClient()
	if err != nil {
		return fmt.Errorf("failed to get Git REST client: %w", err)
	}
	search := &azgit.GitPullRequestSearchCriteria{
		Status:        types.ToPtr(azgit.PullRequestStatusValues.Active),
		SourceRefName: types.ToPtr("refs/heads/" + opts.headBranch),
		TargetRefName: types.ToPtr("refs/heads/" + opts.baseBranch),
	}
	prList, err := restGitClient.GetPullRequests(ctx.Context(), azgit.GetPullRequestsArgs{
		RepositoryId:   types.ToPtr(repo.Id.String()),
		Project:        types.ToPtr(*repo.Project.Name),
		SearchCriteria: search,
		Top:            types.ToPtr(1),
	})
	if err != nil {
		return fmt.Errorf("error checking for existing pull request: %w", err)
	}
	if prList != nil && len(*prList) > 0 {
		existing := (*prList)[0]
		return fmt.Errorf("a pull request for branch %q into branch %q already exists:\n%s", opts.headBranch, opts.baseBranch, *existing.Url)
	}

	azRepo, err := ctx.RepoContext().Repo()
	if err != nil {
		return fmt.Errorf("failed to get repository from context: %w", err)
	}

	identityClient, err := ctx.ClientFactory().Identity(ctx.Context(), azRepo.Organization())
	if err != nil {
		return fmt.Errorf("failed to create Identity client: %w", err)
	}

	prRequest := azgit.GitPullRequest{
		Title:         types.ToPtr(opts.title),
		Description:   types.ToPtr(opts.description),
		SourceRefName: types.ToPtr(fmt.Sprintf("refs/heads/%s", opts.headBranch)),
		TargetRefName: types.ToPtr(fmt.Sprintf("refs/heads/%s", opts.baseBranch)),
		IsDraft:       types.ToPtr(opts.isDraft),
	}

	allReviewers := append(opts.requiredReviewer, opts.optionalReviewer...) //nolint:gocritic
	if len(allReviewers) > 0 {
		descriptors, err := shared.GetReviewerDescriptors(ctx.Context(), identityClient, allReviewers)
		if err != nil {
			return fmt.Errorf("failed to get reviewer descriptors: %w", err)
		}
		var reviewersList []azgit.IdentityRefWithVote
		for i, r := range opts.requiredReviewer {
			reviewersList = append(reviewersList, azgit.IdentityRefWithVote{
				DisplayName: types.ToPtr(r),
				Descriptor:  types.ToPtr(descriptors[i]),
				IsRequired:  types.ToPtr(true),
			})
		}
		offset := len(opts.requiredReviewer)
		for i, r := range opts.optionalReviewer {
			reviewersList = append(reviewersList, azgit.IdentityRefWithVote{
				DisplayName: types.ToPtr(r),
				Descriptor:  types.ToPtr(descriptors[offset+i]),
				IsRequired:  types.ToPtr(false),
			})
		}
		prRequest.Reviewers = &reviewersList
	}

	if opts.dryRun {
		t := template.New(
			iostreams.Out,
			iostreams.TerminalWidth(),
			iostreams.ColorEnabled()).
			WithTheme(iostreams.TerminalTheme()).
			WithFuncs(map[string]any{
				"s": func(v any) string {
					if v == nil {
						return ""
					}

					val := reflect.ValueOf(v)
					if val.Kind() == reflect.Ptr {
						if val.IsNil() {
							return ""
						}
						val = val.Elem()
					}

					if val.Kind() == reflect.String {
						return val.String()
					}

					return ""
				},
				"notBlank": func(s string) bool {
					return strings.TrimSpace(s) != ""
				},
				"stripprefix": strings.TrimPrefix,
			})
		if err := t.Parse(dryRunTpl); err != nil {
			return fmt.Errorf("failed to parse dry-run template: %w", err)
		}

		return t.ExecuteData(prRequest)
	}

	createdPr, err := restGitClient.CreatePullRequest(ctx.Context(), azgit.CreatePullRequestArgs{
		GitPullRequestToCreate: &prRequest,
		RepositoryId:           types.ToPtr(repo.Id.String()),
		Project:                types.ToPtr(*repo.Project.Name),
	})
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}
	fmt.Fprintf(iostreams.Out, "Pull request #%d created: %s\n", *createdPr.PullRequestId, *createdPr.Url)
	return nil
}

func normalizeBranch(b string) string {
	return strings.TrimPrefix(b, "refs/heads/")
}
