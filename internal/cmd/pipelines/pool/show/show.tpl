{{bold "id:"}} {{.Pool.Id}}
{{bold "name:"}} {{s .Pool.Name}}
{{if hasText (s .Pool.PoolType)}}{{bold "type:"}} {{.Pool.PoolType}}
{{end -}}
{{if hasText (u .Pool.Scope)}}{{bold "scope:"}} {{u .Pool.Scope}}
{{end -}}
{{if .Pool.Size}}{{bold "size:"}} {{.Pool.Size}}
{{end -}}
{{if .Pool.IsHosted}}{{bold "is hosted:"}} {{.Pool.IsHosted}}
{{end -}}
{{if .Pool.IsLegacy}}{{bold "is legacy:"}} {{.Pool.IsLegacy}}
{{end -}}
{{if .Pool.AutoProvision}}{{bold "auto provision:"}} {{.Pool.AutoProvision}}
{{end -}}
{{if .Pool.AutoUpdate}}{{bold "auto update:"}} {{.Pool.AutoUpdate}}
{{end -}}
{{if .Pool.CreatedOn}}{{bold "created on:"}} {{timeago .Pool.CreatedOn.Time}} ({{timefmt "2006-01-02 15:04 MST" .Pool.CreatedOn.Time}})
{{end -}}
{{if .Pool.CreatedBy}}{{if hasText (s .Pool.CreatedBy.DisplayName)}}{{bold "created by:"}} {{s .Pool.CreatedBy.DisplayName}} ({{s .Pool.CreatedBy.UniqueName}})
{{end}}{{end -}}
{{if .Pool.Owner}}{{if hasText (s .Pool.Owner.DisplayName)}}{{bold "owner:"}} {{s .Pool.Owner.DisplayName}} ({{s .Pool.Owner.UniqueName}})
{{end}}{{end -}}
