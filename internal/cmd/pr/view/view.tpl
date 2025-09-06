{{bold "url:"}} {{hyperlink .Url .Url}}
{{bold "id:"}} {{.PullRequestId}}
{{bold "title:"}} {{.Title}}
{{bold "author:"}} {{.CreatedBy.DisplayName }} ({{.CreatedBy.UniqueName}})
{{bold "created on:"}} {{timeago .CreationDate.Time }} ({{.CreationDate.Time.Format "2006-01-02 15:04 MST"}})
{{bold "status:"}} {{.Status}}
{{bold "merge status:"}} {{.MergeStatus}}
{{bold "draft:"}} {{.IsDraft}}
{{bold "source branch:"}} {{stripprefix .SourceRefName "refs/heads/" }}
{{bold "target branch:"}} {{stripprefix .TargetRefName "refs/heads/" }}
{{ $reviewers := userReviewers .Reviewers -}}
{{ $length := len $reviewers -}}
{{if gt $length 0 -}}
{{bold "reviewers:"}}
{{range $reviewers}}  {{.DisplayName}} ({{.UniqueName}}): {{vote .Vote}}
{{end -}}
{{end -}}

{{bold "description:" -}}
{{markdown .Description}}
