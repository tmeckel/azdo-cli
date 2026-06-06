{{- if hasText .Url}}
{{bold "url:"}} {{hyperlink .Url .Url}}
{{- end}}
{{- if hasText .Id}}
{{bold "id:"}} {{.Id}}
{{- end}}
{{- if hasText .Name}}
{{bold "name:"}} {{s .Name}}
{{- end}}
{{- if hasText .Description}}
{{bold "description:"}} {{s .Description}}
{{- end}}
{{- if hasText .ProjectName}}
{{bold "project:"}} {{s .ProjectName}} {{if .ProjectId}}({{.ProjectId}}){{end}}
{{- end}}
{{- if hasText .Identity}}
{{bold "identity:"}} {{s .Identity}}
{{- end}}