{{- /* Variable group show template */ -}}
{{ bold "ID:" }} {{ i .Id }}
{{ bold "Name:" }} {{ s .Name }}
{{ bold "Type:" }} {{ s .Type }}
{{ bold "Authorized For All Pipelines:" }} {{ b (authorized) }}
{{- if (hasText .Description) }}
{{ bold "Description:" }} {{ s .Description }}
{{- end }}
{{ bold "Is Shared:" }} {{ b .IsShared }}
{{ bold "Variable Count:" }} {{ len (vars .Variables) }}
{{ bold "Project References:" }} {{ len (projRefs .VariableGroupProjectReferences) }}
{{ bold "Created:" }} {{ ts .CreatedOn }}{{- if .CreatedBy }} by {{ identity .CreatedBy }}{{- end }}
{{ bold "Modified:" }} {{ ts .ModifiedOn }}{{- if .ModifiedBy }} by {{ identity .ModifiedBy }}{{- end }}
{{- $pipelines := pipelines }}
{{- if $pipelines }}
Authorized Pipelines:
{{- range $p := $pipelines }}
  - {{ $p.ID }}{{ if $p.Name }}: {{ $p.Name }}{{ end }}
{{- end }}
{{- end }}
{{- if .Variables }}
{{ bold "Variables:" }}
{{- range $v := vars .Variables }}
   {{ bold $v.Name }}:{{ if $v.ReadOnly }} (read-only){{ end }}{{ if $v.Secret }} (secret){{ end }}{{ if $v.Value }} = {{ s $v.Value }}{{ end }}
{{- end }}
{{- end }}
{{- if .VariableGroupProjectReferences }}
{{ bold "Project References:" }}
{{- range $r := projRefs .VariableGroupProjectReferences }}
  - {{ if $r.ProjectReference }}{{ $r.ProjectReference.Id }}{{ end }}{{ if $r.ProjectReference }}{{ if $r.ProjectReference.Name }}: {{ s $r.ProjectReference.Name }}{{ end }}{{ end }}
{{- end }}
{{- end }}
{{- if (hasAny .ProviderData) }}
{{ bold "Provider Data:" }}
{{- range $k, $v := .ProviderData }}
  {{ bold $k }}: {{ $v }}
{{- end }}
{{- end }}
