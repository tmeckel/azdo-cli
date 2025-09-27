{{bold "Title:"}} {{s .Title}}
{{bold "Source Branch:"}} {{stripprefix (s .SourceRefName) "refs/heads/"}}
{{bold "Target Branch:"}} {{stripprefix (s .TargetRefName) "refs/heads/"}}
{{bold "Draft:"}} {{.IsDraft}}
{{if .Reviewers}}
{{bold "Reviewers:"}}
{{range .Reviewers}}  {{s .DisplayName}} (Required: {{.IsRequired}})
{{end}}
{{end}}

{{bold "Description:"}}
{{if notBlank (s .Description)}}{{markdown (s .Description)}}{{else}}
  None
{{end}}
