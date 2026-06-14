{{- if hasText .Url }}
{{bold "url:"}} {{hyperlink (s .Url) (s .Url)}}
{{- end }}
{{- if hasText .Id }}
{{bold "id:"}} {{.Id}}
{{- end }}
{{- if hasText .BuildNumber }}
{{bold "build number:"}} {{s .BuildNumber}}
{{- end }}
{{- if hasText .Status }}
{{bold "status:"}} {{s .Status}}
{{- end }}
{{- if and (eq (s .Status) "completed") (hasText .Result) }}
{{bold "result:"}} {{s .Result}}
{{- end }}
{{- if hasText .Reason }}
{{bold "reason:"}} {{s .Reason}}
{{- end }}
{{- if and .Definition (hasText (formatEntity .Definition.Name .Definition.Id)) }}
{{bold "definition:"}} {{formatEntity .Definition.Name .Definition.Id}}
{{- end }}
{{- if and .Queue (hasText (formatEntity .Queue.Name .Queue.Id)) }}
{{bold "queue:"}} {{formatEntity .Queue.Name .Queue.Id}}
{{- end }}
{{- if hasText .SourceBranch }}
{{bold "source branch:"}} {{s .SourceBranch}}
{{- end }}
{{- if hasText .SourceVersion }}
{{bold "source version:"}} {{truncate 8 (s .SourceVersion)}}
{{- end }}
{{- if and .RequestedBy (hasText (formatEntity .RequestedBy.DisplayName .RequestedBy.UniqueName)) }}
{{bold "requested by:"}} {{formatEntity .RequestedBy.DisplayName .RequestedBy.UniqueName}}
{{- end }}
{{- if and .RequestedFor (hasText (formatEntity .RequestedFor.DisplayName .RequestedFor.UniqueName)) }}
{{bold "requested for:"}} {{formatEntity .RequestedFor.DisplayName .RequestedFor.UniqueName}}
{{- end }}
{{- if hasText .Priority }}
{{bold "priority:"}} {{s .Priority}}
{{- end }}
{{- if .QueueTime }}
{{bold "queue time:"}} {{timeago .QueueTime.Time}} ({{timefmt "2006-01-02 15:04:05" .QueueTime.Time}})
{{- end }}
{{- if .StartTime }}
{{bold "start time:"}} {{timeago .StartTime.Time}} ({{timefmt "2006-01-02 15:04:05" .StartTime.Time}})
{{- end }}
{{- if .FinishTime }}
{{bold "finish time:"}} {{timeago .FinishTime.Time}} ({{timefmt "2006-01-02 15:04:05" .FinishTime.Time}})
{{- end }}
{{- if and .StartTime .FinishTime }}
{{bold "duration:"}} {{formatDuration .StartTime .FinishTime}}
{{- end }}
{{- if hasItems .Tags }}
{{bold "tags:"}}{{range $i, $tag := .Tags}}{{if gt $i 0}}; {{end}}{{$tag}}{{end}}
{{- end }}
