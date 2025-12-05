{{bold "url:"}} {{hyperlink (s .PullRequest.Url) (s .PullRequest.Url)}}
{{bold "id:"}} {{.PullRequest.PullRequestId}}
{{bold "title:"}} {{s .PullRequest.Title}}
{{bold "author:"}} {{s .PullRequest.CreatedBy.DisplayName }} ({{s .PullRequest.CreatedBy.UniqueName}})
{{bold "created on:"}} {{timeago .PullRequest.CreationDate.Time }} ({{.PullRequest.CreationDate.Time.Format "2006-01-02 15:04 MST"}})
{{bold "status:"}} {{.PullRequest.Status}}
{{bold "merge status:"}} {{.PullRequest.MergeStatus}}
{{bold "draft:"}} {{.PullRequest.IsDraft}}
{{bold "source branch:"}} {{stripprefix (s .PullRequest.SourceRefName) "refs/heads/" }}
{{bold "target branch:"}} {{stripprefix (s .PullRequest.TargetRefName) "refs/heads/" }}
{{ $labels := .PullRequest.Labels -}}
{{ $llength := len $labels -}}
{{if gt $llength 0 -}}
{{bold "labels:"}}
{{range .PullRequest.Labels}}  {{s .Name}}
{{end -}}
{{end -}}
{{ $reviewers := userReviewers .PullRequest.Reviewers -}}
{{ $length := len $reviewers -}}
{{if gt $length 0 -}}
{{bold "reviewers:"}}
{{range $reviewers}}  {{s .DisplayName}} ({{s .UniqueName}}): {{vote .Vote}}
{{end -}}
{{end -}}

{{bold "description:" -}}
{{if .PullRequest.Description}}{{markdown (s .PullRequest.Description)}}{{else}}
  None given
{{end -}}
{{if .Threads}}
{{bold "comments:"}}
{{range .Threads}}
--------------------------------------------------
{{bold "Thread ID:"}} {{.Id}}
{{bold "Status:"}} {{.Status}}
{{- range .Comments}}

{{bold (s .Author.DisplayName)}}{{if notBlank (s .Author.UniqueName)}} ({{s .Author.UniqueName}}){{end}} commented {{timeago .PublishedDate.Time}} (Type: {{s .CommentType}}):
{{markdown (s .Content)}}{{end -}}{{end -}}
{{if .Commits -}}
{{bold "commits:"}}
{{range .Commits}}
{{bold (substr (s .CommitId) 0 7)}} {{s .Author.Name}} ({{timeago .Author.Date.Time}}):
{{markdown (s .Comment)}}{{end}}{{end -}}
{{end -}}
