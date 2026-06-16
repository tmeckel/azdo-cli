{{bold "id:"}} {{.Id}}
{{bold "name:"}} {{s .Name}}
{{if .ProjectId}}{{bold "project id:"}} {{u .ProjectId}}{{end}}
{{if .Pool}}{{bold "pool:"}} {{if .Pool.Id}}{{.Pool.Id}}{{end}}{{if hasText (s .Pool.Name)}} ({{s .Pool.Name}}){{end}}{{end}}
