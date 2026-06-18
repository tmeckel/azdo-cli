{{bold "url:"}} {{if hasText .Node.Url}}{{hyperlink (s .Node.Url) (s .Node.Url)}}{{else}}{{s .Node.Url}}{{end}}
{{bold "id:"}} {{int .Node.Id}}
{{bold "identifier:"}} {{uuid .Node.Identifier}}
{{bold "name:"}} {{s .Node.Name}}
{{bold "path:"}} {{s .Node.Path}}
{{bold "structure:"}} {{s .Node.StructureType}}
{{bold "has children:"}} {{bool .Node.HasChildren}}
{{if .Node.Attributes}}

{{bold "attributes:"}}
{{range $key, $value := .Node.Attributes}}{{printf "%-14s" (printf "%s:" $key)}} {{timefmt "2006-01-02" $value}}
{{end -}}
{{end -}}
{{if .IncludeChildren}}
{{if .Node.Children}}

{{bold "children:"}}
{{range .Node.Children}}  - {{s .Name}}{{if hasText .Identifier}} ({{uuid .Identifier}}){{end}}{{if .HasChildren}} (hasChildren: {{bool .HasChildren}}){{end}}
{{end -}}
{{end -}}
{{end -}}
