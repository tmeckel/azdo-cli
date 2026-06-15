{{- if hasText .Url }}
{{bold "url:"}} {{hyperlink (s .Url) (s .Url)}}
{{- end }}
{{- if hasText .Id }}
{{bold "id:"}} {{.Id}}
{{- end }}
{{- if hasText .Name }}
{{bold "name:"}} {{s .Name}}
{{- end }}
{{- if hasText .Revision }}
{{bold "revision:"}} {{.Revision}}
{{- end }}
{{- if hasText .Path }}
{{bold "path:"}} {{s .Path}}
{{- end }}
{{- if hasText .Type }}
{{bold "type:"}} {{s .Type}}
{{- end }}
{{- if .Process }}
{{bold "process:"}} {{s .Process}}
{{- end }}
{{- if and .Repository (hasText (formatEntity .Repository.Name .Repository.Id)) }}
{{bold "repository:"}} {{formatEntity .Repository.Name .Repository.Id}}
{{- end }}
{{- if and .Queue (hasText (formatEntity .Queue.Name .Queue.Id)) }}
{{bold "queue:"}} {{formatEntity .Queue.Name .Queue.Id}}
{{- end }}
{{- if .AuthoredBy }}
{{bold "authored by:"}} {{identityDisplay .AuthoredBy}}
{{- end }}
{{- if .CreatedDate }}
{{bold "created on:"}} {{timeago .CreatedDate.Time}} ({{timefmt "2006-01-02 15:04:05" .CreatedDate.Time}})
{{- end }}
{{- if hasText .Description }}
{{bold "description:"}}
{{markdown (s .Description)}}
{{- end }}
{{- if hasText .Quality }}
{{bold "quality:"}} {{s .Quality}}
{{- end }}
