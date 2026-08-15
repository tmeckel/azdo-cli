{{bold "url:"}} {{hyperlink (s .WorkItem.Url) (s .WorkItem.Url)}}
{{bold "id:"}} {{.WorkItem.Id}}
{{bold "rev:"}} {{.WorkItem.Rev}}
{{bold "type:"}} {{field .WorkItem "System.WorkItemType"}}
{{bold "state:"}} {{field .WorkItem "System.State"}}
{{bold "reason:"}} {{field .WorkItem "System.Reason"}}
{{bold "title:"}} {{field .WorkItem "System.Title"}}
{{ $assigned := identity .WorkItem "System.AssignedTo" -}}
{{if hasText $assigned}}{{bold "assigned to:"}} {{$assigned}} ({{unique .WorkItem "System.AssignedTo"}})
{{end -}}
{{ $createdBy := identity .WorkItem "System.CreatedBy" -}}
{{if hasText $createdBy}}{{bold "created by:"}} {{$createdBy}} ({{unique .WorkItem "System.CreatedBy"}})
{{end -}}
{{ $createdDate := field .WorkItem "System.CreatedDate" -}}
{{if hasText $createdDate}}{{bold "created on:"}} {{timeago $createdDate}} ({{timefmt "2006-01-02 15:04 MST" $createdDate}})
{{end -}}
{{ $changedDate := field .WorkItem "System.ChangedDate" -}}
{{if hasText $changedDate}}{{bold "changed on:"}} {{timeago $changedDate}} ({{timefmt "2006-01-02 15:04 MST" $changedDate}})
{{end -}}
{{bold "area:"}} {{field .WorkItem "System.AreaPath"}}
{{bold "iteration:"}} {{field .WorkItem "System.IterationPath"}}
{{ $tags := field .WorkItem "System.Tags" -}}
{{if hasText $tags}}{{bold "tags:"}} {{$tags}}
{{end -}}
{{ $priority := field .WorkItem "Microsoft.VSTS.Common.Priority" -}}
{{if hasText $priority}}{{bold "priority:"}} {{$priority}}
{{end -}}
{{ $severity := field .WorkItem "Microsoft.VSTS.Common.Severity" -}}
{{if hasText $severity}}{{bold "severity:"}} {{$severity}}
{{end -}}

{{bold "description:" -}}
{{if .Description}}{{markdown .Description}}{{else}}
  None given
{{end -}}
{{if .Relations}}{{with .WorkItem.Relations}}
{{bold "relations:"}}
{{range .}}  - {{s .Rel}}: {{hyperlink (s .Url) (s .Url)}}
{{end -}}
{{end}}{{end -}}
{{if .Comments}}
{{bold "comments:"}}
{{range .Comments}}
--------------------------------------------------
{{bold (cauthor .)}} commented {{timeago (cdate .)}}:
{{markdown (s .Text)}}{{end -}}
{{end -}}
