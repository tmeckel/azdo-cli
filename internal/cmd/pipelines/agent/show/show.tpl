{{- $a := .Agent -}}
{{- with $a.Links -}}
{{- $self := index . "self" -}}
{{- with $self -}}
{{- $href := index . "href" -}}
{{- with $href -}}
{{bold "url:"}} {{hyperlink . .}}
{{- end -}}
{{- end -}}
{{- end -}}
{{if hasText $a.Id}}{{bold "id:"}} {{$a.Id}}{{end}}
{{if hasText $a.Name}}{{bold "name:"}} {{$a.Name}}{{end}}
{{bold "pool:"}} {{.PoolName}}
{{if hasText $a.Status}}{{bold "status:"}} {{$a.Status}}{{end}}
{{if hasText $a.Enabled}}{{bold "enabled:"}} {{$a.Enabled}}{{end}}
{{if hasText $a.Version}}{{bold "version:"}} {{$a.Version}}{{end}}
{{if hasText $a.OsDescription}}{{bold "os description:"}} {{$a.OsDescription}}{{end}}
{{if hasText $a.AccessPoint}}{{bold "access point:"}} {{$a.AccessPoint}}{{end}}
{{if hasText $a.MaxParallelism}}{{bold "max parallelism:"}} {{$a.MaxParallelism}}{{end}}
{{if $a.CreatedOn}}{{bold "created on:"}} {{timeago $a.CreatedOn.Time}} ({{$a.CreatedOn.Time.Format "2006-01-02 15:04 MST"}}){{end}}
{{if $a.StatusChangedOn}}{{bold "last status change:"}} {{timeago $a.StatusChangedOn.Time}} ({{$a.StatusChangedOn.Time.Format "2006-01-02 15:04 MST"}}){{end}}
{{- if .IncludeCapabilities -}}
{{- if $a.SystemCapabilities -}}
{{bold "system capabilities:"}}
{{- range $key, $val := $a.SystemCapabilities}}
  {{$key}}: {{$val}}
{{- end -}}
{{- end -}}
{{- if $a.UserCapabilities -}}
{{bold "user capabilities:"}}
{{- range $key, $val := $a.UserCapabilities}}
  {{$key}}: {{$val}}
{{- end -}}
{{- end -}}
{{- end -}}
