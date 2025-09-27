package view

import (
	_ "embed"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/identity"
	"github.com/spewerspew/spew"
	"github.com/spf13/cobra"
	"github.com/tmeckel/azdo-cli/internal/cmd/pr/shared"
	"github.com/tmeckel/azdo-cli/internal/cmd/util"
	"github.com/tmeckel/azdo-cli/internal/template"
)

type viewOptions struct {
	exporter     util.Exporter
	selectorArg  string
	showComments bool
	showCommits  bool
	showRaw      bool
	commentType  string
	commentSort  string
}

//go:embed view.tpl
var vtempl string

type templateData struct {
	PullRequest *git.GitPullRequest
	Threads     *[]git.GitPullRequestCommentThread
	Commits     *[]git.GitCommitRef
}

type pullRequestJSON struct {
	URL          *string         `json:"url,omitempty"`
	ID           *int            `json:"id,omitempty"`
	Title        *string         `json:"title,omitempty"`
	Author       *authorJSON     `json:"author,omitempty"`
	CreatedOn    *time.Time      `json:"createdOn,omitempty"`
	Status       string          `json:"status,omitempty"`
	MergeStatus  string          `json:"mergeStatus,omitempty"`
	IsDraft      *bool           `json:"isDraft,omitempty"`
	SourceBranch *string         `json:"sourceBranch,omitempty"`
	TargetBranch *string         `json:"targetBranch,omitempty"`
	Reviewers    *[]reviewerJSON `json:"reviewers,omitempty"`
	Description  *string         `json:"description,omitempty"`
	Threads      *[]threadJSON   `json:"threads,omitempty"`
	Commits      *[]commitJSON   `json:"commits,omitempty"`
}

type authorJSON struct {
	DisplayName *string `json:"displayName,omitempty"`
	UniqueName  *string `json:"uniqueName,omitempty"`
}

type reviewerJSON struct {
	DisplayName *string `json:"displayName,omitempty"`
	UniqueName  *string `json:"uniqueName,omitempty"`
	Vote        string  `json:"vote,omitempty"`
}

type threadJSON struct {
	Id       *int           `json:"id,omitempty"`
	Status   *string        `json:"status,omitempty"`
	Comments *[]commentJSON `json:"comments,omitempty"`
}

type commentJSON struct {
	Author      *authorJSON `json:"author,omitempty"`
	PublishedOn *time.Time  `json:"publishedOn,omitempty"`
	Type        *string     `json:"type,omitempty"`
	Content     *string     `json:"content,omitempty"`
}

type commitJSON struct {
	CommitID *string     `json:"commitId,omitempty"`
	Author   *authorJSON `json:"author,omitempty"`
	Date     *time.Time  `json:"date,omitempty"`
	Comment  *string     `json:"comment,omitempty"`
}

func voteToString(vote *int) (string, error) {
	if vote == nil {
		return "unknown", nil
	}
	switch *vote {
	case 10:
		return "approved", nil
	case 5:
		return "approved with suggestions", nil
	case 0:
		return "no vote", nil
	case -5:
		return "waiting for author", nil
	case -10:
		return "rejected", nil
	}
	return "", fmt.Errorf("unknown vote %d", *vote)
}

func NewCmd(ctx util.CmdContext) *cobra.Command {
	opts := &viewOptions{}

	cmd := &cobra.Command{
		Use:   "view [<number> | <branch> | <url>]",
		Short: "View a pull request",
		Long: heredoc.Docf(`
			Display the title, body, and other information about a pull request.

			Without an argument, the pull request that belongs to the current branch
			is displayed.
		`, "`"),
		Args: cobra.MaximumNArgs(1),
		Aliases: []string{
			"show",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.selectorArg = args[0]
			}
			return runCmd(ctx, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.showComments, "comments", "c", false, "View pull request comments")
	cmd.Flags().BoolVarP(&opts.showCommits, "commits", "C", false, "View pull request commits")
	cmd.Flags().BoolVarP(&opts.showRaw, "raw", "r", false, "View pull request raw")
	util.StringEnumFlag(cmd, &opts.commentType, "comment-type", "", "text", []string{"text", "system", "all"}, "Filter comments by type; defaults to 'text'")
	util.StringEnumFlag(cmd, &opts.commentSort, "comment-sort", "", "desc", []string{"desc", "asc"}, "Sort comments by creation time; defaults to 'desc' (newest first)")
	util.AddFormatFlags(cmd, &opts.exporter)

	return cmd
}

func runCmd(ctx util.CmdContext, opts *viewOptions) (err error) {
	iostreams, err := ctx.IOStreams()
	if err != nil {
		return err
	}
	finder, err := shared.NewFinder(ctx)
	if err != nil {
		return err
	}
	pr, repo, err := finder.Find(shared.FindOptions{
		Selector: opts.selectorArg,
	})
	if err != nil {
		return err
	}

	if pr == nil {
		return fmt.Errorf("pr is nil")
	}

	var threads *[]git.GitPullRequestCommentThread
	clientFactory := ctx.ClientFactory()

	if opts.showComments {
		gitClient, err := clientFactory.Git(ctx.Context(), repo.Organization())
		if err != nil {
			return err
		}

		repoIdStr := pr.Repository.Id.String()
		threads, err = gitClient.GetThreads(ctx.Context(), git.GetThreadsArgs{
			RepositoryId:  &repoIdStr,
			PullRequestId: pr.PullRequestId,
			Project:       pr.Repository.Project.Name,
		})
		if err != nil {
			return err
		}

		if threads != nil && len(*threads) > 0 {
			sort.SliceStable(*threads, func(i, j int) bool {
				iDate := (*threads)[i].PublishedDate
				jDate := (*threads)[j].PublishedDate

				if opts.commentSort == "asc" {
					if iDate == nil {
						return true
					}
					if jDate == nil {
						return false
					}
					return iDate.Time.Before(jDate.Time)
				}

				// desc
				if iDate == nil {
					return false
				}
				if jDate == nil {
					return true
				}
				return iDate.Time.After(jDate.Time)
			})
		}

		// Filter comments by type
		if threads != nil && opts.commentType != "all" {
			filteredThreads := make([]git.GitPullRequestCommentThread, 0)
			for _, thread := range *threads {
				if thread.Comments == nil {
					continue
				}
				filteredComments := make([]git.Comment, 0)
				for _, comment := range *thread.Comments {
					if comment.CommentType != nil && string(*comment.CommentType) == opts.commentType {
						filteredComments = append(filteredComments, comment)
					}
				}
				if len(filteredComments) > 0 {
					thread.Comments = &filteredComments
					filteredThreads = append(filteredThreads, thread)
				}
			}
			threads = &filteredThreads
		}

		if threads != nil {
			guidsToResolve := make(map[string]struct{})
			re := regexp.MustCompile(`@<([a-fA-F0-9-]{36})>`)

			for _, thread := range *threads {
				if thread.Comments == nil {
					continue
				}
				for _, comment := range *thread.Comments {
					if comment.Content == nil {
						continue
					}
					matches := re.FindAllStringSubmatch(*comment.Content, -1)
					for _, match := range matches {
						if len(match) > 1 {
							guidsToResolve[strings.ToLower(match[1])] = struct{}{}
						}
					}
				}
			}

			guidToName := make(map[string]string)
			if len(guidsToResolve) > 0 {
				var identityIds []uuid.UUID
				for guidStr := range guidsToResolve {
					guid, err := uuid.Parse(guidStr)
					if err == nil {
						identityIds = append(identityIds, guid)
					}
				}

				if len(identityIds) > 0 {
					identityClient, err := clientFactory.Identity(ctx.Context(), repo.Organization())
					if err != nil {
						return err
					}

					identities, err := identityClient.ReadIdentityBatch(ctx.Context(), identity.ReadIdentityBatchArgs{
						BatchInfo: &identity.IdentityBatchInfo{
							IdentityIds: &identityIds,
						},
					})
					if err == nil && identities != nil {
						for _, id := range *identities {
							if id.Id != nil && id.ProviderDisplayName != nil {
								guidToName[id.Id.String()] = *id.ProviderDisplayName
							}
						}
					}
				}
			}

			if len(guidToName) > 0 {
				for i := range *threads {
					if (*threads)[i].Comments == nil {
						continue
					}
					for j := range *(*threads)[i].Comments {
						comment := &(*(*threads)[i].Comments)[j]
						if comment.Content == nil {
							continue
						}
						newContent := re.ReplaceAllStringFunc(*comment.Content, func(match string) string {
							guid := strings.ToLower(re.FindStringSubmatch(match)[1])
							if name, ok := guidToName[guid]; ok {
								return "**@" + name + "**"
							}
							return match
						})
						comment.Content = &newContent
					}
				}
			}
		}
	}

	var commits *[]git.GitCommitRef
	if opts.showCommits {
		gitClient, err := clientFactory.Git(ctx.Context(), repo.Organization())
		if err != nil {
			return err
		}

		repoIdStr := pr.Repository.Id.String()

		commitsResponse, err := gitClient.GetPullRequestCommits(ctx.Context(), git.GetPullRequestCommitsArgs{
			RepositoryId:  &repoIdStr,
			PullRequestId: pr.PullRequestId,
			Project:       pr.Repository.Project.Name,
		})
		if err != nil {
			return err
		}
		commits = &commitsResponse.Value
	}

	if opts.showRaw {
		spew.Dump(pr)
		if threads != nil {
			spew.Dump(threads)
		}
		if commits != nil {
			spew.Dump(commits)
		}
		return nil
	}

	if opts.exporter != nil {
		prJSON := &pullRequestJSON{
			URL:         pr.Url,
			ID:          pr.PullRequestId,
			Title:       pr.Title,
			IsDraft:     pr.IsDraft,
			Description: pr.Description,
		}
		if pr.Status != nil {
			prJSON.Status = string(*pr.Status)
		}
		if pr.MergeStatus != nil {
			prJSON.MergeStatus = string(*pr.MergeStatus)
		}
		if pr.SourceRefName != nil {
			sb := strings.TrimPrefix(*pr.SourceRefName, "refs/heads/")
			prJSON.SourceBranch = &sb
		}
		if pr.TargetRefName != nil {
			tb := strings.TrimPrefix(*pr.TargetRefName, "refs/heads/")
			prJSON.TargetBranch = &tb
		}
		if pr.CreatedBy != nil {
			prJSON.Author = &authorJSON{
				DisplayName: pr.CreatedBy.DisplayName,
				UniqueName:  pr.CreatedBy.UniqueName,
			}
		}
		if pr.CreationDate != nil {
			prJSON.CreatedOn = &pr.CreationDate.Time
		}
		if pr.Reviewers != nil {
			reviewers := make([]reviewerJSON, 0)
			for _, r := range *pr.Reviewers {
				if r.IsContainer != nil && *r.IsContainer {
					continue
				}
				voteStr, _ := voteToString(r.Vote)
				reviewers = append(reviewers, reviewerJSON{
					DisplayName: r.DisplayName,
					UniqueName:  r.UniqueName,
					Vote:        voteStr,
				})
			}
			prJSON.Reviewers = &reviewers
		}
		if threads != nil {
			threadsJSON := make([]threadJSON, 0)
			for _, thread := range *threads {
				if thread.Comments == nil {
					continue
				}
				commentsJSON := make([]commentJSON, 0)
				for _, comment := range *thread.Comments {
					var commentAuthor *authorJSON
					if comment.Author != nil {
						commentAuthor = &authorJSON{
							DisplayName: comment.Author.DisplayName,
							UniqueName:  comment.Author.UniqueName,
						}
					}
					var publishedDate *time.Time
					if comment.PublishedDate != nil {
						publishedDate = &comment.PublishedDate.Time
					}
					var commentType *string
					if comment.CommentType != nil {
						ct := string(*comment.CommentType)
						commentType = &ct
					}

					commentsJSON = append(commentsJSON, commentJSON{
						Author:      commentAuthor,
						PublishedOn: publishedDate,
						Type:        commentType,
						Content:     comment.Content,
					})
				}
				if len(commentsJSON) > 0 {
					threadsJSON = append(threadsJSON, threadJSON{
						Id:       thread.Id,
						Status:   (*string)(thread.Status),
						Comments: &commentsJSON,
					})
				}
			}
			prJSON.Threads = &threadsJSON
		}
		if commits != nil {
			commitsJSON := make([]commitJSON, 0)
			for _, c := range *commits {
				var commitAuthor *authorJSON
				if c.Author != nil {
					commitAuthor = &authorJSON{
						DisplayName: c.Author.Name,
					}
				}
				var commitDate *time.Time
				if c.Author != nil && c.Author.Date != nil {
					commitDate = &c.Author.Date.Time
				}
				commitsJSON = append(commitsJSON, commitJSON{
					CommitID: c.CommitId,
					Author:   commitAuthor,
					Date:     commitDate,
					Comment:  c.Comment,
				})
			}
			prJSON.Commits = &commitsJSON
		}

		iostreams, err := ctx.IOStreams()
		if err != nil {
			return err
		}
		return opts.exporter.Write(iostreams, prJSON)
	}

	t := template.New(
		iostreams.Out,
		iostreams.TerminalWidth(),
		iostreams.ColorEnabled()).
		WithTheme(iostreams.TerminalTheme()).
		WithFuncs(map[string]any{
			"substr": func(s string, start, length int) string {
				if start < 0 {
					start = 0
				}
				if start >= len(s) {
					return ""
				}
				end := start + length
				if end > len(s) {
					end = len(s)
				}
				return s[start:end]
			},
			"notBlank": func(s string) bool {
				return strings.TrimSpace(s) != ""
			},
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
			"userReviewers": func(reviewers *[]git.IdentityRefWithVote) (*[]git.IdentityRefWithVote, error) {
				rl := []git.IdentityRefWithVote{}
				if len(*reviewers) > 0 {
					for _, r := range *reviewers {
						if r.IsContainer != nil && *r.IsContainer {
							continue
						}
						rl = append(rl, r)
					}
				}
				return &rl, nil
			},
			"vote": voteToString,
		})
	err = t.Parse(vtempl)
	if err != nil {
		return err
	}

	data := templateData{
		PullRequest: pr,
		Threads:     threads,
		Commits:     commits,
	}

	return t.ExecuteData(data)
}
