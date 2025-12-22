{{bold "ID:"}} {{u .Id}}
{{if hasText .Name -}}
{{bold "Name:"}} {{s .Name}}
{{end -}}
{{if hasText .Type -}}
{{bold "Type:"}} {{s .Type}}
{{end -}}
{{if hasText .Description -}}
{{bold "Description:"}} {{s .Description}}
{{end -}}
{{if hasText .Owner -}}
{{bold "Owner:"}} {{s .Owner}}
{{end -}}
{{bold "IsReady:"}} {{b .IsReady}}
{{bold "IsShared:"}} {{b .IsShared}}
{{if hasText .Url -}}
{{bold "URL:"}} {{s .Url}}
{{end -}}
{{if .CreatedBy -}}
{{bold "Created By:"}} {{identity .CreatedBy}}
{{end -}}
{{with .Data -}}
{{bold "Data:"}}
{{range $key, $value := . -}}
{{printf "  %s:" $key | bold}} {{$value}}
{{end -}}
{{end -}}
{{if .Authorization -}}
{{bold "Authorization:"}}
{{with .Authorization -}}
{{if hasText .Scheme -}}
{{bold "  Scheme:"}} {{scheme .}}
{{end -}}
{{with .Parameters -}}
{{range $key, $value := . -}}
{{printf "  %s:" $key | bold}} {{$value}}
{{end -}}
{{end -}}
{{end -}}
{{end -}}
